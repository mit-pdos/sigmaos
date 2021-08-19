#!/bin/bash

#
# Run from directory thas has "bin"
#

N=":1111"
if [ $# -eq 1 ]
then
    N=$1
fi

if [[ -z "${NAMED}" ]]; then
  export NAMED=$N
fi

#strace -fc ./bin/kernel/memfsd 0 ":1111" 2> strace.txt &
./bin/kernel/memfsd 0 ":1111" 2> memfsd.err &

sleep 1

./bin/kernel/nps3d &
#strace -f ./bin/kernel/npuxd 2> strace.txt &
./bin/kernel/npuxd 2> npuxd.err &
./bin/kernel/procd ./ &

sleep 2
./mount.sh
mkdir -p /mnt/9p/fs   # make fake file system
mkdir -p /mnt/9p/kv
mkdir -p /mnt/9p/gg
mkdir -p /mnt/9p/memfsd
