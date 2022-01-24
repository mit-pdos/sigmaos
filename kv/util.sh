#!/bin/bash
PID=$1
SLEEP_TIME=1 # seconds
HZ=100       # ticks/second
prev_ticks=0
while true; do
    sfile=$(cat /proc/$PID/stat)

    utime=$(awk '{print $14}' <<< "$sfile")
    stime=$(awk '{print $15}' <<< "$sfile")
    ticks=$(($utime + $stime))

    echo $ticks $utime $stime

    pcpu=$(bc <<< "scale=4 ; ($ticks - $prev_ticks) / ($HZ * $SLEEP_TIME) * 100")

    echo "util:" $pcpu

    prev_ticks="$ticks"

    sleep $SLEEP_TIME
done
