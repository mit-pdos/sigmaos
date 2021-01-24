#!/bin/sh

for i in {0..100}
do
    echo "$i" > /mnt/9p/kvd/$i
done
