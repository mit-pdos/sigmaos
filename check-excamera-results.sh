#!/bin/bash

memfs_dir=/mnt/9p/fs/gg

if [ ! -d "/mnt/9p/fs" ]
then
  echo "9p not mounted!"
  exit 1
fi

exc_dir=$HOME/gg/examples/excamera

cd $exc_dir

echo "1. Copying results to local dir"
cp -r /mnt/9p/fs/gg/results/* .

rm mylist.txt

ls *-vpxenc.ivf | while read each; do echo "file '$each'" >> mylist.txt; done
ffmpeg -f concat -i mylist.txt -codec copy output.avi
file output.avi
