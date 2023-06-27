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
  sudo sed -i s/GRUB_CMDLINE_LINUX_DEFAULT=.*/GRUB_CMDLINE_LINUX_DEFAULT="\"intel_pstate=passive intel_idle.max_cstate=0 systemd.unified_cgroup_hierarchy=1\""/g  /etc/default/grub
  sudo update-grub
  sudo reboot
ENDSSH
done
