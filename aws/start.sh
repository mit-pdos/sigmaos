#!/bin/bash

if [ "$#" -ne 1 ]
then
    echo "Usage: vpc-id"
    exit 1
fi

vms=`./lsvpc.py $1 | grep -w VMInstance | cut -d " " -f 5`

vma=($vms)
NAME="${vma[0]}"
NAMED="${vma[0]}:1111"
export NAMED="${NAMED}"
echo "NAME": $NAME

for vm in $vms
do
    echo "START: $vm"
    ssh -i key-$1.pem ubuntu@$vm <<ENDSSH
    ssh-agent bash -c 'ssh-add ~/.ssh/aws-ulambda; (cd ulambda; git pull)'
    ./ulambda/stop.sh
    (cd ulambda; ./make.sh)
    if [ "${vm}" = "${NAME}" ]; then 
       echo "START NAMED"
       nohup ./ulambda/bin/named > named.out 2>&1 < /dev/null &
    fi
    echo "NAMED: " ${NAMED}
    export NAMED="${NAMED}"
    nohup ./ulambda/bin/nps3d > npsd3.out 2>&1 < /dev/null &
    # ./ulambda/bin/npuxd &
ENDSSH
done

#
# open up TCP port to NAMED machine to make mount work
#

#../umount.sh
#echo "start proxy: $NAMED"
#../bin/proxyd &
#sudo mount -t 9p -o tcp,name=`whoami`,uname=`whoami`,port=1110 127.0.0.1 /mnt/9p
#ls /mnt/9p
