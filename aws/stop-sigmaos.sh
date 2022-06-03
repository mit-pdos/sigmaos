#!/bin/bash

if [ "$#" -ne 1 ]
then
  echo "Usage: vpc-id"
  exit 1
fi

VPC=$1
vms=`./lsvpc.py $VPC | grep -w VMInstance | cut -d " " -f 5`

for vm in $vms
do
    echo "stop: $vm"
    ssh -i key-$VPC.pem ubuntu@$vm /bin/bash <<ENDSSH
    (cd ulambda; ./stop.sh)
    rm -rf /tmp/ulambda/h
ENDSSH
done
