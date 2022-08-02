#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC --command COMMAND" 1>&2
}

VPC=""
COMMAND=""
while [[ $# -gt 0 ]]; do
  case $1 in
  --vpc)
    shift
    VPC=$1
    shift
    ;;
  --command)
    shift
    COMMAND=$1
    shift
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


if [ -z "$VPC" ] || [ -z "$COMMAND" ] || [ $# -gt 0 ]; then
  usage
  exit 1
fi

vms=(`./lsvpc.py $VPC | grep -w VMInstance | cut -d " " -f 5`)
vms_privaddr=(`./lsvpc.py $VPC --privaddr | grep -w VMInstance | cut -d " " -f 6`)
MAIN="${vms[0]}"
MAIN_PRIVADDR="${vms_privaddr[0]}"

echo "Run [$MAIN]: $COMMAND"
ssh -i key-$VPC.pem ubuntu@$MAIN /bin/bash <<ENDSSH
cd ulambda
export NAMED=$MAIN_PRIVADDR:1111
export SIGMAPERF="KVCLERK_TPT;MRMAPPER_TPT;MRREDUCER_TPT;TEST_TPT;"
$COMMAND
ENDSSH
