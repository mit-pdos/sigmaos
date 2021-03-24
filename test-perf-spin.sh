#!/bin/bash

# Params
dim=100
max_its=50 # Step size = 10
baseline_its=200
N=1

# Dirs
measurements=./measurements
baseline=$measurements/spin_test_baseline.txt

echo "Spinning perf test, dimension=$dim max iterations=$max_its"
if [ ! -d "$measurements" ]
then
  mkdir $measurements
fi

echo "Collecting basline..."
echo $dim $baseline_its 1 > $baseline
./bin/perf-matrix-multiply-baseline $dim $baseline_its >> $baseline 2>&1

for i in `seq 1 $max_its`
do
  its=$(($i * 10))

  echo "Restarting 9p infrastructure..."
  ./stop.sh
  ./start.sh
  sleep 1
  
  echo "Starting spin test, spinners=$N, iterations=$its"
  echo $dim $its $N > $measurements/spin_test_${dim}_${its}_${N}
  ./bin/perf-spin-test-starter $N $dim $its >> $measurements/spin_test_${dim}_${its}_${N} 2>&1
done
