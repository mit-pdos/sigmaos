#!/bin/bash

COUNT=50
folder="../05140026-logs"
mkdir -p "$folder"
echo "$folder"
for ((i = 1; i <= COUNT; i++)); do
  echo "[$(date)] Run #$i"
  go test sigmaos/scontainer --run TestPythonStat --start
  logfile="$folder/Run${i}.log"
  ./logs.sh > "$logfile"
  ./stop.sh --parallel --nopurge --skipdb; go clean -testcache  
done
