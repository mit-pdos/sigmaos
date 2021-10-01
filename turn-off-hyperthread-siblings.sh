#!/bin/bash

siblings=$(perf/get_hyperthread_siblings.py)
siblings_array=($siblings)

for i in "${siblings_array[@]}"
do
  echo "Turning off core $i"
  echo 0 | sudo tee /sys/devices/system/cpu/cpu$i/online
done
