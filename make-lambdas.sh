#!/bin/sh

# create ulambdas

L="/mnt/9p/ulambda/"
mappers=( )

# PID name program input status
makeLambda () {
    echo "makeDir" $1 $2 $3 $4
    PID=$((1 + $RANDOM % 1000000))
    mkdir "${L}${PID}"
    touch "${L}${PID}/$1"
    echo $2 > "${L}${PID}/program"
    echo $3 > "${L}${PID}/input"
    touch "${L}${PID}/$4"
}

O="/mnt/9p/fs/mr-wc"
mkdir -p ${O}

# setup mappers
i=0
for f in /mnt/9p/fs/pg*.txt
do
    makeLambda "Mapper" "./bin/mr-m-wc" "name/fs/$O/" "Runnable"
    mappers+=( ${PID} )
    i=$((i+1))
done

echo "mappers" ${mappers[@]}

# setup reducers
i=0
while [  $i -lt 1 ]; 
do
    mkdir -p  ${O}/$i
    makeLambda "Reducer" "./bin/mr-r-wc" "name/fs/$O/$i" "Waiting" 
    echo ${mappers[@]} > "${L}${PID}/dependencies"
    i=$((i+1))
done



# cat ~/classes/6824-2021/6.824-golabs-staff/mygo/src/main/pg-*.txt | tr -s '[[:punct:][:space:]]' '\n' | sort | less
