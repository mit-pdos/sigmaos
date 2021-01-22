#!/bin/sh

# Create a wc mapreduce job:
# $ ./mk-wc-job.sh | ./bin/submit

L="/mnt/9p/ulambd/dev"
O="/mnt/9p/fs/mr-wc"
N=1

mkdir -p ${O}

# set up output dir
i=0
while [  $i -lt $N ]; 
do
    mkdir -p  "${O}/$i"
    i=$((i+1))
done

# setup mappers and readers, as a paired dep
mappers=()
i=0
for f in ./input/pg-*.txt
do
    RPID=$((1 + $RANDOM % 1000000))
    MPID=$((1 + $RANDOM % 1000000))
    
    args=("$f"  "$i")
    pairs=("(${RPID};${MPID})")
    echo "$RPID","./bin/fsreader","[${args[@]}]","[${pairs[@]}]","[]"

    args=("name/$i/pipe"  "$i")
    echo "$MPID","./bin/mr-m-wc","[${args[@]}]","[${pairs[@]}]","[]"
    mappers+=( ${MPID} )

    i=$((i+1))
done

# setup reducers
i=0
while [  $i -lt $N ]; 
do
    PID=$((1 + $RANDOM % 1000000))
    args=("name/fs/mr-wc/$i" "name/fs/mr-wc/mr-out" )
    echo $PID,"./bin/mr-r-wc","[${args[@]}]","[]","[${mappers[@]}]"
    i=$((i+1))
done

# cat input/pg-*.txt | tr -s '[[:punct:][:space:]]' '\n' | sort | less
