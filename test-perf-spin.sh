#!/bin/bash

# Params
dim=15
max_its=100 # Step size = 5
n_trials=25
baseline_its=10000
N=1

# Dirs
measurements=./measurements
baseline=$measurements/spin_test_baseline.txt

echo "Spinning perf test, dimension=$dim max iterations per lambda invocation=$max_its"
if [ ! -d "$measurements" ]
then
  mkdir $measurements
fi

echo "Collecting basline..."
echo $dim $baseline_its 1 > $baseline
./bin/perf-matrix-multiply-baseline $dim $baseline_its >> $baseline 2>&1

for i in `seq 1 $max_its`
do
  for j in `seq 1 $n_trials`
  do
    its=$(($i * 5))

    echo "Restarting 9p infrastructure..."
    ./stop.sh
    ./start.sh
    sleep 1
    
    echo "Starting spin test, spinners=$N, iterations=$its"
    echo $dim $its $N > $measurements/spin_test_${dim}_${its}_${N}_${j}.txt
    ./bin/perf-spin-test-starter $N $dim $its >> $measurements/spin_test_${dim}_${its}_${N}_${j}.txt 2>&1
  done
done
