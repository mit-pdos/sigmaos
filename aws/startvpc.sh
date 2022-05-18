#!/bin/bash

if [ "$#" -ne 1 ]
then
    echo "Usage:[-c] vpc-id"
    exit 1
fi

COMPILE="no"
VPC=""
while [[ $# -gt 0 ]]
do
    key="$1"
    case $key in
	-c)
            COMPILE="yes"
            shift
            ;;
	*)
            VPC=$1
            shift
            break
            ;;
    esac
done

vms=`./lsvpc.py $1 | grep -w VMInstance | cut -d " " -f 5`

vma=($vms)
MAIN="${vma[0]}"
NAMED="${vma[0]}:1111"
export NAMED="${NAMED}"

for vm in $vms
do
    echo "UPDATE: $vm"
    ssh -i key-$1.pem ubuntu@$vm /bin/bash <<ENDSSH
    ssh-agent bash -c 'ssh-add ~/.ssh/aws-ulambda; (cd ulambda; git pull > /tmp/git.out )'
    (cd ulambda; ./stop.sh)
ENDSSH
done

for vm in $vms
do
    echo "COMPILE: $vm"
    ( ssh -i key-$1.pem ubuntu@$vm /bin/bash <<ENDSSH
    grep "+++" /tmp/git.out && (cd ulambda; ./make.sh -norace > /tmp/make.out 2>&1 )  
ENDSSH
      ) &
done

wait
echo "COMPILES DONE"

for vm in $vms
do
    scp -i key-$1.pem ubuntu@$vm:/tmp/make.out /dev/stdout
done

for vm in $vms
do
    ssh -i key-$1.pem ubuntu@$vm /bin/bash <<ENDSSH
    export NAMED="${NAMED}"
    if [ "${vm}" = "${MAIN}" ]; then 
       echo "START ${NAMED}"
       (cd ulambda; nohup ./start.sh > /tmp/out 2>&1 < /dev/null &)
    else
       echo "JOIN ${NAMED}"
       (cd ulambda;  nohup bin/realm/noded . $vm > /tmp/out 2>&1 < /dev/null &)
    fi
ENDSSH
done
