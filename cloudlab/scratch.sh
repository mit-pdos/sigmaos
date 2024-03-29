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
#    sudo apt install -y cpufrequtils
#    git checkout etcd-sigmasrv-newprocclnt
#    git pull
    
#    sudo apt install -y apparmor-utils
#    (cd sigmaos; sudo apparmor_parser -r container/sigmaos-uproc )
#    (cd sigmaos; ./set-cores.sh --set 1 --start 4 --end 39 )
#    sudo rm -rf /data/volumes/*
aa-status | grep sigmaos
ENDSSH
done
