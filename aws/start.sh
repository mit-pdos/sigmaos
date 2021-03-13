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
    ./ulambda/stop.sh
    (cd ulambda; ./make.sh)
    if [ "${vm}" = "${MAIN}" ]; then 
       echo "START NAMED"
       nohup ./ulambda/bin/named > named.out 2>&1 < /dev/null &
       nohup ./ulambda/bin/proxyd > proxyd.out 2>&1 < /dev/null &
       nohup ./ulambda/bin/schedd > schedd.out 2>&1 < /dev/null &
    fi
    nohup ./ulambda/bin/nps3d > npsd3.out 2>&1 < /dev/null &
    nohup ./ulambda/bin/npuxd > nnpuxd.out 2>&1 < /dev/null &
    nohup ./ulambda/bin/locald > locald.out 2>&1 < /dev/null &
ENDSSH
done

# HACK
ssh -i key-$1.pem -L 1110:localhost:1110 ubuntu@${MAIN} sleep 999999999

#
# Run in another window:
#
# sudo mount -t 9p -o tcp,name=`whoami`,uname=`whoami`,port=1110 ${IP} /mnt/9p
