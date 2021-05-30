#!/bin/bash

./make_norace.sh

dim=64
its=100
secs_per_test=20

measurements=./measurements/arrival-process
measurements_latency=$measurements/latency
measurements_util=$measurements/util
measurements_pprof=$measurements/pprof
mkdir -p $measurements
mkdir -p $measurements_latency
mkdir -p $measurements_util
mkdir -p $measurements_pprof


for spawns_per_sec in 10 20 30 40 50 60 70 80 90 100
do
  echo "Stopping any currently running 9p infrastructure..."
  ./stop.sh
  sleep 2

  echo "Starting 9p infrastructure"
  ./bin/memfsd 0 ":1111" $measurements_pprof/$spawns_per_sec-memfsd-pprof.out $measurements_util/$spawns_per_sec-memfsd-util.txt &
  sleep 2
  ./bin/locald ./ $measurements_pprof/$spawns_per_sec-locald-pprof.out $measurements_util/$spawns_per_sec-locald-util.txt &
  sleep 2

  echo "Spawning $spawns_per_sec spinners per second"
  ./bin/rival $spawns_per_sec $secs_per_test ninep $dim $its 2> $measurements_latency/$spawns_per_sec-latency.txt
done

./stop.sh
sleep 2
