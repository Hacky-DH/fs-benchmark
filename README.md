# fs-benchmark
This benchmark supports read-write and metadata performance test
on the wide distribute file system, such as [ceph](https://ceph.com), [moosefs](https://moosefs.com), [lizardfs](https://lizardfs.org/) and so on
and generates test graphs using pandas.

# read-write test
This test uses [iozone](http://iozone.org/) to run the read-write test
and includes write, read, randread, randwrite.

## default parameters
```
block size(KiB): 8 16 32 64 128 256 512 1024 2048
concurrent: 1 2 4 8 16
file size: 1GiB
```
## run and plot
```bash
cd /path/to/mount/fs
NAME=test TEST_LOOP=3 DIRECT="" bash $ROOT/rw-benchmark/iozone/perftest.sh | tee /tmp/testlog
# the result files is in $HOME dir
python $ROOT/rw-benchmark/plot/plot.py -f $HOME/iozone-test-<date>-{}.tar.gz -r 3
```
# metadata test
metadata test tests the performance of MDS of ceph, or master of moosefs, or other fs
```
Usage of perftest:
  -b uint
        start index of operations
  -c uint
        concurrent of operations (default 1)
  -id uint
        client id (default 1)
  -interval duration
        interval to calculate tps, e.g. 1h5m8s (default 1m0s)
  -n string
        count string of operations, k,m,g (default "10")
  -period duration
        period duration of each stages, e.g. 1h5m8s (default 5m0s)
  -s string
        count of operations in one dir, k,m,g (default "10")
  -t    for test
  -v    show version
```
## Example
```
cd /path/to/mount/fs
$ROOT/meta-benchmark/bin/perftest -n 3m -c 100 -id 1 -interval 1m -period 3h
-n 3,000,000 files
-c 100 concurrent
-id 1
-interval 1 minute
-period 3 hour
```
