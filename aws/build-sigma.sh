#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC --realm REALM [--force-build]" 1>&2
}

VPC=""
REALM=""
FORCE=""
while [[ $# -gt 0 ]]; do
  case $1 in
  --vpc)
    shift
    VPC=$1
    shift
    ;;
  --realm)
    shift
    REALM=$1
    shift
    ;;
  --force-build)
    shift
    FORCE="--force-build"
    ;;
  -help)
    usage
    exit 0
    ;;
  *)
    echo "Error: unexpected argument '$1'"
    usage
    exit 1
    ;;
  esac
done


if [ -z $VPC ] || [ -z $REALM ] || [ $# -gt 0 ]; then
  usage
  exit 1
fi

vms=(`./lsvpc.py $VPC | grep -w VMInstance | cut -d " " -f 5`)

MAIN="${vms[0]}"

echo "UPDATE: $MAIN"
ssh -i key-$VPC.pem ubuntu@$MAIN /bin/bash <<ENDSSH
ssh-agent bash -c 'ssh-add ~/.ssh/aws-ulambda; (cd ulambda; git pull > /tmp/git.out 2>&1 )'
(cd ulambda; ./stop.sh)
# Make sure we build the first time sigmaos is installed.
if [ -f ~/.nobuild ] || ! [ -z "$FORCE" ]; then
  echo "" > /tmp/git.out
fi
ENDSSH

ssh -i key-$VPC.pem ubuntu@$MAIN /bin/bash <<ENDSSH
! grep "Already up to date." /tmp/git.out && echo "COMPILE: $MAIN" && (cd ulambda; ./make.sh --norace --target aws > /tmp/make.out 2>&1;)  
echo "UPLOAD: $MAIN"
(cd ulambda; ./upload.sh --realm $REALM;)
# NOte that we have completed the build on this machine at least once.
if [ -f ~/.nobuild ]; then
  rm ~/.nobuild
fi
ENDSSH
