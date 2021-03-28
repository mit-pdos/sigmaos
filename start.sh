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

./bin/named &
./bin/schedd &
./bin/nps3d &
./bin/npuxd &

# start NPROC locald instances
NPROC=`nproc`
for ((i = 0 ; i < $NPROC ; i++)); do
  ./bin/locald ./ &
done

sleep 2
./mount.sh
mkdir -p /mnt/9p/fs   # make fake file system
mkdir -p /mnt/9p/kv
mkdir -p /mnt/9p/gg
