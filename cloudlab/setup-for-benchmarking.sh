#!/bin/bash

usage() {
  echo "Usage: $0 [--turbo] address" 1>&2
}

NOTURBO="true"
ADDR="NONE"
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --turbo)
    shift
    NOTURBO="false"
    ;;
  -help)
    usage
    exit 0
    ;;
  *)
    ADDR=$1
    shift
    ;;
  esac
done

if [ "$#" -ne 0 ] || [ "$ADDR" == "NONE" ];
then
  usage
  exit 1
fi

DIR=$(dirname $0)
source $DIR/env.sh

export NOTURBO=$NOTURBO

CMD=$(
envsubst '$NOTURBO' <<'ENDSSH'
# Turn off turbo boost.
if [ "${NOTURBO}" == "true" ]; then
  echo "Turning off turbo boost"
  echo 1 | sudo tee /sys/devices/system/cpu/intel_pstate/no_turbo
else
  echo "Turning on turbo boost"
  echo 0 | sudo tee /sys/devices/system/cpu/intel_pstate/no_turbo
fi
# Disable CPU frequency scaling.
# sudo cpupower frequency-set -g performance
np=$(nproc)
np=$((np-1))
for i in $(seq 0 $np) 
do
  echo "CPU frequency set core $i"
  sudo cpufreq-set -g performance -c $i
done
ENDSSH
)

#ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$ADDR <<'ENDSSH'
ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$ADDR <<ENDSSH
  $CMD
ENDSSH

echo "== TO LOGIN TO VM INSTANCE USE: =="
echo "ssh $ADDR"
echo "============================="
