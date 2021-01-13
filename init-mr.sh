#!/bin/sh

# XXX cd /mnt/9p/name?

# setup mr
mkdir -p /mnt/9p/mr
mkdir -p /mnt/9p/mr/todo
mkdir -p /mnt/9p/mr/started
mkdir -p /mnt/9p/mr/reduce

# setup inputs
i=0
echo "/mnt/9p/fs/mrwc" > "/mnt/9p/mr/program"
for f in /mnt/9p/fs/pg*.txt
do
    echo "name/fs/`basename $f`" > "/mnt/9p/mr/todo/task$i"
    i=$((i+1))
done

    
