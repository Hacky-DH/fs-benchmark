package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	bufsz = 1024
)

var (
	test       bool
	version    bool
	id         uint
	concurrent uint64
	begin      uint64
	count      string
	segcount   string
	versionstr string
	period     time.Duration
	interval   time.Duration
)

func init() {
	flag.UintVar(&id, "id", 1, "client id")
	flag.Uint64Var(&concurrent, "c", 1, "concurrent of operations")
	flag.Uint64Var(&begin, "b", 0, "start index of operations")
	flag.StringVar(&count, "n", "10", "count string of operations, k,m,g")
	flag.StringVar(&segcount, "s", "10", "count of operations in one dir, k,m,g")
	flag.BoolVar(&test, "t", false, "for test")
	flag.BoolVar(&version, "v", false, "show version")
	flag.DurationVar(&period, "period", 5*time.Minute,
		"period duration of each stages, e.g. 1h5m8s")
	flag.DurationVar(&interval, "interval", time.Minute,
		"interval to calculate tps, e.g. 1h5m8s")
}

func checkErr(err error) {
	if err != nil {
		log.Fatal("Error: ", err)
	}
}

func mainRecover() {
	if err := recover(); err != nil {
		log.Fatal("Error: ", err)
	}
}

// convert string 10m to 10000000
// support k m g
func convert(str string) uint64 {
	var res uint64 = 0
	if len(str) > 0 {
		str = strings.ToLower(str)
		if strings.LastIndexAny(str, "kmg") != -1 {
			tmp, err := strconv.Atoi(str[:len(str)-1])
			checkErr(err)
			res = uint64(tmp)
			switch str[len(str)-1] {
			case 'k':
				res *= 1000
			case 'm':
				res *= 1000 * 1000
			case 'g':
				res *= 1000 * 1000 * 1000
			}
		} else {
			tmp, err := strconv.Atoi(str)
			checkErr(err)
			res = uint64(tmp)
		}
	}
	return res
}

type stage struct {
	cb     func(name string) error
	isRand bool
}

type perftest struct {
	sum        uint64
	dirs       uint64
	seg        uint64
	concurrent uint64
	rbuf       []byte
	wbuf       []byte
	stages     map[string]stage
}

func newPerftest() *perftest {
	sum := convert(count)
	var dirs, seg, cc uint64
	if concurrent > 1 {
		cpu := uint64(runtime.NumCPU())
		if concurrent > cpu {
			concurrent = cpu
		}
		if sum%concurrent != 0 {
			sum = (sum/concurrent + 1) * concurrent
		}
		seg = sum / concurrent
		dirs, cc = concurrent, concurrent
	} else {
		seg = convert(segcount)
		if sum%seg != 0 {
			sum = (sum/seg + 1) * seg
		}
		dirs = sum / seg
		cc = 1
	}
	data := make([]byte, bufsz)
	_, err := rand.Read(data)
	checkErr(err)
	rand.Seed(time.Now().Unix())
	p := &perftest{sum: sum,
		seg:        seg,
		dirs:       dirs,
		concurrent: cc,
		rbuf:       make([]byte, bufsz),
		wbuf:       data,
	}
	p.stages = make(map[string]stage)
	p.stages["create_write"] = stage{p.create_write, false}
	p.stages["open_read"] = stage{p.open_read, true}
	p.stages["open"] = stage{p.open, true}
	p.stages["utime"] = stage{p.utime, true}
	p.stages["rename"] = stage{p.rename, false}
	p.stages["unlink"] = stage{p.unlink, false}
	return p
}

func (p *perftest) String() string {
	return fmt.Sprintf("%d concurrent %d files in %d dirs with %d segs",
		p.concurrent, p.sum, p.dirs, p.seg)
}

func (p *perftest) work(stageName string, s stage) {
	var exit, max, min, cur, last uint64
	min = math.MaxUint64
	checkTps := time.NewTicker(interval)
	defer checkTps.Stop()
	stopTimer := time.NewTimer(period)
	defer stopTimer.Stop()
	start := time.Now()
	early := make(chan uint64, p.dirs)
	var wg sync.WaitGroup
	wg.Add(int(p.dirs))
	for d := uint64(0); d < p.dirs; d++ {
		dir := fmt.Sprintf("client%d/dir%010d", id, d)
		err := os.MkdirAll(dir, 0755)
		checkErr(err)
		go func(dir string) {
			defer wg.Done()
			next := begin
			for {
				if atomic.LoadUint64(&exit) > 0 {
					return
				}
				if s.isRand {
					next = uint64(rand.Int63n(int64(p.seg)))
				} else {
					next++
					if next >= p.seg {
						early <- 1
						return
					}
				}
				name := fmt.Sprintf("%s/file%010d", dir, next)
				s.cb(name)
				atomic.AddUint64(&cur, 1)
			}
		}(dir)
	}
	defer func() {
		wg.Wait()
		elap := time.Now().Sub(start)
		if min == math.MaxUint64 {
			min = 0
		}
		log.Printf("%14s %14.3f %14.3f %14.3f %-v", stageName,
			float64(max)/interval.Seconds(),
			float64(min)/interval.Seconds(),
			float64(cur)/elap.Seconds(), elap)
	}()
	for {
		select {
		case <-early:
			return
		case <-checkTps.C:
			c := atomic.LoadUint64(&cur)
			cc := c - last
			if cc > max {
				max = cc
			}
			if cc < min {
				min = cc
			}
			last = c
		case <-stopTimer.C:
			atomic.StoreUint64(&exit, 1)
			return
		}
	}
}

func (p *perftest) run() {
	for s, f := range p.stages {
		p.work(s, f)
	}
}

func (p *perftest) clean() {
	dir := fmt.Sprintf("client%d", id)
	os.RemoveAll(dir)
}

func (p *perftest) create_write(name string) error {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_SYNC, 0644)
	checkErr(err)
	_, err = f.Write(p.wbuf)
	checkErr(err)
	err = f.Close()
	checkErr(err)
	return nil
}

func (p *perftest) open_read(name string) error {
	f, err := os.Open(name)
	defer f.Close()
	if err != nil {
		return err
	}
	_, err = f.Read(p.rbuf)
	return err
}

func (p *perftest) open(name string) error {
	f, err := os.Open(name)
	defer f.Close()
	return err
}

func (p *perftest) utime(name string) error {
	now := time.Now()
	err := os.Chtimes(name, now, now)
	return err
}

func (p *perftest) rename(name string) error {
	err := os.Rename(name, name+"_rename")
	return err
}

func (p *perftest) unlink(name string) error {
	err := os.Remove(name)
	if err != nil {
		err = os.Remove(name + "_rename")
	}
	return err
}

func main() {
	defer mainRecover()
	flag.Parse()
	if version {
		fmt.Println(versionstr)
		return
	}

	p := newPerftest()
	log.Printf("client%d period %v %s", id, period, p)
	if test {
		return
	}
	log.Printf("%15s %14s %14s %14s %10s", "stage",
		"max_tps", "min_tps", "avg_tps", "period")
	p.run()
	p.clean()
}
