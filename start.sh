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

./bin/memfsd 0 ":1111" &
./bin/nps3d &
./bin/npuxd &
./bin/locald ./ &

sleep 2
./mount.sh
mkdir -p /mnt/9p/fs   # make fake file system
mkdir -p /mnt/9p/kv
mkdir -p /mnt/9p/gg
mkdir -p /mnt/9p/memfsd-replicas

# Start a few memfs replicas
config_path_linux=/mnt/9p/memfs-replica-config.txt
config_path_9p=name/memfs-replica-config.txt
printf "192.168.0.36:30001\n192.168.0.36:30002\n192.168.0.36:30003\n" > $config_path_linux
./bin/memfs-replica 1 ":30001" $config_path_9p &
./bin/memfs-replica 2 ":30002" $config_path_9p &
./bin/memfs-replica 3 ":30003" $config_path_9p &
