#!/bin/bash

vms=(`./lsvpc.py $VPC | grep -w VMInstance | cut -d " " -f 5`)

for vm in $vms; do
  echo "UPDATE: $vm"
  ssh -i key-$VPC.pem ubuntu@$vm /bin/bash <<ENDSSH
  (cd ulambda; ./stop.sh; ./install.sh -from s3)
ENDSSH
done
