#!/bin/bash

if [ "$#" -ne 1 ]
then
    echo "Usage: vpc-id"
    exit 1
fi

vms=`./lsvpc.py $1 | grep -w VMInstance | cut -d " " -f 5`
for vm in $vms
do
    ssh -i key-$1.pem ubuntu@$vm <<ENDSSH    
    killall named
    killall nps3d
    killall npuxd
ENDSSH
done    
killall proxyd
sudo umount /mnt/9p
