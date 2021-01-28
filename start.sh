#!/bin/sh

./bin/named &
./bin/schedd &
./mount.sh
# make fake file system
mkdir -p /mnt/9p/fs
mkdir -p /mnt/9p/kv
mkdir -p /mnt/9p/gg

#cp ~/classes/6824-2021/6.824-golabs-staff/mygo/src/main/pg-*.txt /mnt/9p/fs

