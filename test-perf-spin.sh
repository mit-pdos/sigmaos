#!/bin/bash

# Params
dim=64
max_its=30 # Step size = 5
n_trials=50
baseline_its=30000
remote_baseline_its=5000
N=1

# Dirs
measurements=./measurements
native_baseline=$measurements/spin_test_native_baseline.txt
remote_baseline=$measurements/spin_test_remote_baseline.txt

echo "Spinning perf test, dimension=$dim max iterations per lambda invocation=$max_its"
if [ ! -d "$measurements" ]
then
  mkdir $measurements
fi

echo "Collecting native baseline..."
echo $dim $baseline_its 1 > $native_baseline
./bin/perf-spin-test-starter 1 $dim $baseline_its baseline local >> $native_baseline 2>&1

echo "Warming up aws lambda..."
for k in {1..50} ; do
  echo "Aws warmup round $k..."
  ./bin/perf-spin-test-starter $N $dim 20 aws remote >> /dev/null 2>&1
done

echo "Collecting remote baseline..."
echo $dim $remote_baseline_its 1 > $remote_baseline
./bin/perf-spin-test-starter 1 $dim $remote_baseline_its baseline remote >> $remote_baseline 2>&1

for test_type in native 9p aws ; do
  ./stop.sh
  echo "Running $test_type tests..."
  if [ $test_type == "9p" ]; then
    echo "Starting 9p infrastructure..."
    ./start.sh
    sleep 1
  fi

  if [ $test_type == "aws" ]; then
    # Warm up
    echo "Warming up aws lambda..."
    for k in {1..50} ; do
      echo "Aws warmup round $k..."
      ./bin/perf-spin-test-starter $N $dim 20 $test_type remote >> /dev/null 2>&1
    done
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
      if [ $test_type == "aws" ]; then
        ./bin/perf-spin-test-starter $N $dim $its $test_type remote >> $outfile 2>&1
      else
        ./bin/perf-spin-test-starter $N $dim $its $test_type local >> $outfile 2>&1
      fi
    done
  done
done
