#!/bin/sh

./bin/named &
./bin/schedd &
./bin/nps3d &
./mount.sh
mkdir -p /mnt/9p/fs   # make fake file system
mkdir -p /mnt/9p/kv
mkdir -p /mnt/9p/gg

