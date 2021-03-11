#!/bin/bash

if [ "$#" -ne 1 ]
then
    echo "Usage: vpc-id"
    exit 1
fi

vms=`./lsvpc.py $1 | grep -w VMInstance | cut -d " " -f 5`
N=0
for vm in $vms
do
    echo "start $vm"
    if [ "${N}" -eq 0 ]; then
	echo "set named"
	export NAMED=$vm:1111
    fi
    ssh -i key-$1.pem ubuntu@$vm <<ENDSSH
    ssh-agent bash -c 'ssh-add ~/.ssh/aws-ulambda; (cd ulambda; git pull)'
    # (cd ulambda; ./make.sh)
    if [[ "${N}" -eq "0" ]]; then 
        ./ulambda/bin/named &
    fi
    echo "NAMED: " ${NAMED}
    export NAMED="${NAMED}"
    ./ulambda/bin/nps3d &
    # ./ulambda/bin/npuxd &
    N=1
    echo "N: " $N
ENDSSH
done
echo "start proxy: $NAMED"
../bin/proxyd &
sudo mount -t 9p -o tcp,name=`whoami`,uname=`whoami`,port=1110 127.0.0.1 /mnt/9p
ls /mnt/9p
