#!/bin/bash

if [ "$#" -ne 1 ]
then
  echo "Usage: $0 user@address"
  exit 1
fi

echo "$0 $1"

DIR=$(dirname $0)

. $DIR/config

ssh -i $DIR/keys/cloudlab-sigmaos $1 <<'ENDSSH'
# Turn off turbo boost.
echo 1 | sudo tee /sys/devices/system/cpu/intel_pstate/no_turbo

# Disable CPU frequency scaling.
sudo cpupower frequency-set -g performance
ENDSSH

echo "== TO LOGIN TO VM INSTANCE USE: =="
echo "ssh $1"
echo "============================="
