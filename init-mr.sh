#!/bin/sh

# XXX cd /mnt/9p/name?

# setup inputs
i=0
echo "/mnt/9p/fs/mrwc" > "/mnt/9p/mr/program"
for f in /mnt/9p/fs/pg*.txt
do
    echo "name/fs/`basename $f`" > "/mnt/9p/mr/map/task$i"
    i=$((i+1))
done

# cat ~/classes/6824-2021/6.824-golabs-staff/mygo/src/main/pg-*.txt | tr -s '[[:punct:][:space:]]' '\n' | sort | less
