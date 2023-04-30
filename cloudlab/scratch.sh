#!/bin/bash

LOGIN="arielck"
DIR=$(dirname $0)

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
    sudo swapoff -a
    sudo rm -f /swapfile
ENDSSH
done
