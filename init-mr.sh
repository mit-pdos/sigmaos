#!/bin/sh

# XXX cd /mnt/9p/name?

# setup inputs
i=0
echo "/mnt/9p/fs/mrwc" > "/mnt/9p/mr/program"
for f in /mnt/9p/fs/pg*.txt
do
    echo "name/fs/`basename $f`" > "/mnt/9p/mr/todo/task$i"
    i=$((i+1))
done

    
