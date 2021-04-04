#!/bin/bash

# Params
dim=64
max_its=100 # Step size = 5
n_trials=50
baseline_its=30000
N=1

# Dirs
measurements=./measurements
baseline=$measurements/spin_test_baseline.txt

echo "Spinning perf test, dimension=$dim max iterations per lambda invocation=$max_its"
if [ ! -d "$measurements" ]
then
  mkdir $measurements
fi

echo "Collecting baseline..."
echo $dim $baseline_its 1 > $baseline
./bin/perf-spin-test-starter 1 $dim $baseline_its baseline >> $baseline 2>&1

for test_type in native 9p ; do
  ./stop.sh
  echo "Running $test_type tests..."
  if [ $test_type == "9p" ]; then
    echo "Starting 9p infrastructure..."
    ./start.sh
    sleep 1
  fi
   
  for i in `seq 1 $max_its`
  do
    for j in `seq 1 $n_trials`
    do
      outfile=$measurements/spin_test_${dim}_${its}_${N}_${j}_$test_type.txt
      its=$(($i * 5))
  
      # Don't redo work
      if [ -f "$outfile" ]; then
        continue
      fi
     
      echo "Starting spin test, spinners=$N, iterations=$its, trial=$j, type=$test_type"
      echo $dim $its $N > $outfile
      ./bin/perf-spin-test-starter $N $dim $its $test_type >> $outfile 2>&1
    done
  done
done
