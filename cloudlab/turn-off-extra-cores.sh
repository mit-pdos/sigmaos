#!/bin/bash

DIR=$(dirname $0)
source $DIR/env.sh

vms=`cat servers.txt | cut -d " " -f2` 

vma=($vms)
MAIN="${vma[0]}"
MAIN_PRIVADDR=$(./leader-ip.sh) 
#export SIGMANAMED="${SIGMANAMED}"

if ! [ -z "$N_VM" ]; then
  vms=${vma[@]:0:$N_VM}
fi

if [ ! -z "$TAG" ]; then
  ./update-repo.sh --parallel --branch jaeger # docker-dev
fi

vm_ncores=$(ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$MAIN nproc)

for vm in $vms; do
  echo $vm
  ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm <<ENDSSH
  cd sigmaos
  ./set-cores.sh --set 0 --start 4 --end $(($vm_ncores - 1))
ENDSSH
done
