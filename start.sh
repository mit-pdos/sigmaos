#!/bin/bash

#
# Run from directory thas has "bin"
#

N=":1111"
if [ $# -eq 1 ]
then
    N=$1
fi

if [[ -z "${NAMED}" ]]; then
  export NAMED=$N
fi

if [[ -z "${N_REPLICAS}" ]]; then
  export N_REPLICAS=1
fi

echo "running with NAMED=$NAMED"
echo "each realm runs with $N_REPLICAS replicas"

#./bin/kernel/boot
./bin/realm/realmmgr . &
sleep 2
./bin/realm/realmd . $(hostname) &
sleep 1
./bin/realm/create 1000

./mount.sh
