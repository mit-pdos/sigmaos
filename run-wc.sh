#!/bin/sh

# Create ulambdas for wc mapreduce

L="/mnt/9p/ulambd/ulambd"
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


# from stackoverflow
json_array() {
  echo -n '['
  while [ $# -gt 0 ]; do
    x=${1//\\/\\\\}
    echo -n \"${x//\"/\\\"}\"
    [ $# -gt 1 ] && echo -n ', '
    shift
  done
  echo ']'
}

# Create a json struct matching ulamd.Attr
spawnLambda () {
    echo "spawnLambda" $1 $2 $3 $4

    PID=$((1 + $RANDOM % 1000000))
    echo "Spawn ${PID} { \"Program\": \"$1\", \"Args\": "$2", \"AfterStart\": "$3", \"AfterExit\": "$4" }" >> ${L}
}

# setup mappers and readers
mappers=()
i=0
for f in ~/classes/6824-2021/golabs-staff/mygo/src/main/pg-*.txt
do
    args=("$f"  "$i")
    args=`json_array "${args[@]}"`
    spawnLambda "./bin/fsreader" "$args" "[]" "[]"
    rpid=${PID}

    args=("name/$i/pipe"  "$i")
    args=`json_array "${args[@]}"`
    afterstart=(${rpid})
    afterstart=`json_array "${afterstart[@]}"`
    spawnLambda "./bin/mr-m-wc" "$args" "$afterstart" "[]"
    mappers+=( ${PID} )

    i=$((i+1))
done

# reducers don't run until all mappers completed
afterexit=`json_array "${mappers[@]}"`

# setup reducers
i=0
while [  $i -lt $N ]; 
do
    args=("name/fs/mr-wc/$i" "name/fs/mr-wc/mr-out" )
    args=`json_array "${args[@]}"`
    spawnLambda "./bin/mr-r-wc" "$args" "[]" "$afterexit"
    i=$((i+1))
done

# cat ~/classes/6824-2021/6.824-golabs-staff/mygo/src/main/pg-*.txt | tr -s '[[:punct:][:space:]]' '\n' | sort | less
