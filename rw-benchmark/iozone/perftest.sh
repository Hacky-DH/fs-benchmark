#!/bin/bash
set -e
# use iozone to perftest fs, read and write
# depends env:
# URL url to download iozone
# HOME home dir
# NAME name of perftest
# TEST_LOOP number of perftest loop
# DIRECT I use directIO, "" no use
# example
# wget http://url/perftest.sh -qO - | NAME=test TEST_LOOP=3 DIRECT="" bash | tee /tmp/testlog
start_date=`date +%Y%m%d`
TEST_LOOP=${TEST_LOOP:-3}
if [[ ! -x $HOME/iozone ]];then
    wget ${URL}/iozone -qO $HOME/iozone
    chmod +x $HOME/iozone
fi
NAME=${NAME:-test}
for i in $(seq 1 $TEST_LOOP);do
    dir=$HOME/iozone-${NAME}-${start_date}-${i}
    mkdir -p $dir
    for r in 8 16 32 64 128 256 512 1024 2048;do
        for t in 1 2 4 8 16;do
            echo "$(date +%Y%m%d-%H%M) IOZONE test loop: ${i}, block size:${r}KB, Threads: ${t}"
            $HOME/iozone -e${DIRECT} -r${r} -t${t} -s1g -i0 -i1 -i2 -R > $dir/iozone-r${r}-t${t}-$(date +%Y%m%d-%H%M).log
            sleep 60
        done
    done
    sdir=$(basename $dir)
    tar czf $sdir.tgz $sdir -C $HOME
    # upload to server
    # sshpass -p pwd scp -o StrictHostKeyChecking=no $dir.tgz user@host:/path
done
