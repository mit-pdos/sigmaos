#!/bin/bash

if [ "$#" -ne 1 ]
then
    echo "Usage: vpc-id"
    exit 1
fi

vms=`./lsvpc.py $1 | grep -w VMInstance | cut -d " " -f 5`

vma=($vms)
MAIN="${vma[0]}"

echo "LOGIN" ${MAIN}

ssh -i key-$1.pem -L 1110:localhost:1110 ubuntu@${MAIN} sleep 999999999 
