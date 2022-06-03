#!/bin/bash

if [ "$#" -ne 1 ]
then
  echo "Usage: vpc-id"
  exit 1
fi

VPC=$1

vms=(`./lsvpc.py $VPC | grep -w VMInstance | cut -d " " -f 5`)

MAIN="${vms[0]}"

echo "UPDATE: $MAIN"
ssh -i key-$VPC.pem ubuntu@$MAIN /bin/bash <<ENDSSH
ssh-agent bash -c 'ssh-add ~/.ssh/aws-ulambda; (cd ulambda; git pull > /tmp/git.out 2>&1 )'
(cd ulambda; ./stop.sh)
ENDSSH

echo "COMPILE AND UPLOAD: $MAIN"
ssh -i key-$VPC.pem ubuntu@$MAIN /bin/bash <<ENDSSH
grep "+++" /tmp/git.out && (cd ulambda; ./make.sh -norace -target aws > /tmp/make.out 2>&1; ./upload.sh )  
ENDSSH
