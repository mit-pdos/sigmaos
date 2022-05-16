#!/bin/bash

if [ "$#" -ne 1 ]
then
    echo "Usage: vpc-id"
    exit 1
fi

vms=`./lsvpc.py $1 | grep -w VMInstance | cut -d " " -f 5`

vma=($vms)
MAIN="${vma[0]}"
NAMED="${vma[0]}:1111"
export NAMED="${NAMED}"

for vm in $vms
do
    echo "START: $vm"
    ssh -i key-$1.pem ubuntu@$vm <<ENDSSH
    export NAMED="${NAMED}"
    ssh-agent bash -c 'ssh-add ~/.ssh/aws-ulambda; (cd ulambda; git pull)'
    (cd ulambda; ./stop.sh)
    (cd ulambda; ./make.sh -norace)
    if [ "${vm}" = "${MAIN}" ]; then 
       echo "START NAMED"
       nohup ./start.sh > /tmp/out 2>&1 < /dev/null &
    else
       echo "JOIN ${NAMED}"
       (cd ulambda;  nohup bin/realm/noded . $vm > /tmp/out 2>&1 < /dev/null &)
    fi
ENDSSH
done
