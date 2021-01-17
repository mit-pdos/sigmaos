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
    echo "makeDir" $1 $2 $3 $4

    PID=$((1 + $RANDOM % 1000000))
    mkdir "${L}${PID}"
    echo "{ \"Program\": \"$1\", \"Args\": "$2", \"Dependencies\": "$3" }" > "${L}${PID}/attr"
    touch "${L}${PID}/$4"
}

O="/mnt/9p/fs/mr-wc"
mkdir -p ${O}

# setup mappers
mappers=()
i=0
for f in /mnt/9p/fs/pg*.txt
do
    args=("name/fs/`basename $f`"  "$i")
    args=`json_array "${args[@]}"`
    makeLambda "./bin/mr-m-wc" "$args" "[]" "Runnable"
    mappers+=( ${PID} )
    i=$((i+1))
done

deps=`json_array "${mappers[@]}"`

# setup reducers
i=0
while [  $i -lt 1 ]; 
do
    mkdir -p  "${O}/$i"
    args=("name/fs/mr-wc/$i" "name/fs/mr-wc/mr-out" )
    args=`json_array "${args[@]}"`
    makeLambda "./bin/mr-r-wc" "$args" "$deps" "Waiting"
    i=$((i+1))
done

# echo "Start" > /mnt/9p/ulambd/ulambd

# cat ~/classes/6824-2021/6.824-golabs-staff/mygo/src/main/pg-*.txt | tr -s '[[:punct:][:space:]]' '\n' | sort | less
