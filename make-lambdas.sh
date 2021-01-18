#!/bin/sh

# Create ulambdas for wc mapreduce

L="/mnt/9p/ulambd/pids/"
mappers=( )

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
makeLambda () {
    echo "makeDir" $1 $2 $3 $4 $5

    PID=$((1 + $RANDOM % 1000000))
    mkdir "${L}${PID}"
    echo "{ \"Program\": \"$1\", \"Args\": "$2", \"AfterStart\": "$3", \"AfterExit\": "$4" }" > "${L}${PID}/attr"
    touch "${L}${PID}/$5"
}

O="/mnt/9p/fs/mr-wc"
mkdir -p ${O}

# setup mappers and readers
mappers=()
i=0
for f in ~/classes/6824-2021/6.824-golabs-staff/mygo/src/main/pg-*.txt
do
    args=("$f"  "$i")
    args=`json_array "${args[@]}"`
    makeLambda "./bin/fsreader" "$args" "[]" "[]" "Runnable"
    rpid=${PID}

    args=("$i/pipe"  "$i")
    args=`json_array "${args[@]}"`
    afterstart=(${rpid})
    afterstart=`json_array "${before[@]}"`
    makeLambda "./bin/mr-m-wc" "$args" "$afterstart" "[]" "Waiting"
    mappers+=( ${PID} )

    i=$((i+1))
done

# reducers don't run until all mappers completed
afterexit=`json_array "${mappers[@]}"`

# setup reducers
i=0
while [  $i -lt 1 ]; 
do
    mkdir -p  "${O}/$i"
    args=("name/fs/mr-wc/$i" "name/fs/mr-wc/mr-out" )
    args=`json_array "${args[@]}"`
    makeLambda "./bin/mr-r-wc" "$args" "[]" "$afterexit" "Waiting"
    i=$((i+1))
done

# echo "Start" > /mnt/9p/ulambd/ulambd

# cat ~/classes/6824-2021/6.824-golabs-staff/mygo/src/main/pg-*.txt | tr -s '[[:punct:][:space:]]' '\n' | sort | less
