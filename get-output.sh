#!/bin/bash

echo "Copy test out..."
cp /tmp/out /tmp/out2

echo "Scrape logs..."
./logs.sh > /tmp/out1 2>&1 ; grep -E "FATAL|panic|ERROR" /tmp/out1

echo "List all writes"
grep 'ALWAYS Write ' /tmp/out > /tmp/out3

echo "Grep for mismatched write begin/ends..."
echo "====="
for i in 0 1 2 3 4 ; do
  echo "Writes for writer $i:"
  echo "  nwrite begin $i: $(grep -E "Write start .*-$i" /tmp/out | wc -l)" 
  echo "  nwrite end $i:   $(grep -E "Write done .*-$i" /tmp/out | wc -l)" 
  FINAL_FD=$(grep -E "Write done .*-$i" /tmp/out | tail -n1 | cut -d " " -f7)
  echo "    final write FD $i: $FINAL_FD"
done
echo "====="

echo "Find sigmaclntsrv sigmaclnt IDs..."
for cid in $(grep -E "SIGMACLNTSRV.*Write sigmaclntsrv" /tmp/out1 | cut -d " " -f4 | sort -u | tr -d ":") ; do
  echo "Sigmaclntsrv sigmaclnt ID: $cid"
  SESS_ID=$(grep -E " fsuxd-.*Add cid $cid sess" /tmp/out1 | sort -k 1 | tail -n1 | cut -d " " -f8)
  echo "  sessid: $SESS_ID"
  echo "  nwrite begin $cid: $(grep -E "SIGMACLNTSRV $cid: Write sigmaclntsrv begin" /tmp/out1 | wc -l)" 
  echo "  nwrite end $cid: $(grep -E "SIGMACLNTSRV $cid: Write sigmaclntsrv returned" /tmp/out1 | wc -l)" 
  FINAL_FD=$(grep -E "SIGMACLNTSRV $cid: Write sigmaclntsrv returned" /tmp/out1 | tail -n1 | cut -d " " -f8)
  echo "    final write FD $cid: $FINAL_FD"
done
