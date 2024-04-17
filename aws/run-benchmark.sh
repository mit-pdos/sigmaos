#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC --command COMMAND [--vm VM]" 1>&2
}

VPC=""
VM=0
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
  --vm)
    shift
    VM=$1
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

vms=(`./lsvpc.py $VPC | grep '.amazonaws.com' | grep -w VMInstance | cut -d " " -f 5`)
vms_privaddr=(`./lsvpc.py $VPC --privaddr | grep '.amazonaws.com' | grep -w VMInstance | cut -d " " -f 6`)
MAIN="${vms[0]}"
MAIN_PRIVADDR="${vms_privaddr[0]}"

SSHVM="${vms[$VM]}"

# Get the pubkey and privkey for the SigmaOS deployment
MASTER_PUB_KEY="$(ssh -i key-$VPC.pem ubuntu@$MAIN cat /tmp/sigmaos/master-key.pub)"
MASTER_PRIV_KEY="$(ssh -i key-$VPC.pem ubuntu@$MAIN cat /tmp/sigmaos/master-key.priv)"

echo "Run [$SSHVM]: $COMMAND"
ssh -i key-$VPC.pem ubuntu@$SSHVM /bin/bash <<ENDSSH
  # Make sure swap is off on the benchmark machines.
  ulimit -n 100000
  sudo swapoff -a 
  cd sigmaos
  source ./env/env.sh
  echo "$MASTER_PRIV_KEY" > /tmp/sigmaos/master-key.priv 
  echo "$MASTER_PUB_KEY" > /tmp/sigmaos/master-key.pub
  export SIGMAPERF="KVCLERK_TPT;MRMAPPER_TPT;MRREDUCER_TPT;HOTEL_WWW_TPT;TEST_TPT;BENCH_TPT;THUMBNAIL_TPT;"
#  export SIGMAPERF="KVCLERK_TPT;MRMAPPER_TPT;MRREDUCER_TPT;HOTEL_WWW_TPT;TEST_TPT;BENCH_TPT;THUMBNAIL_TPT;MRREDUCER_PPROF;MRMAPPER_PPROF;MRREDUCER_PPROF_MUTEX;MRMAPPER_PPROF_MUTEX;"
#  export SIGMAPERF="KVCLERK_TPT;MRMAPPER_TPT;MRREDUCER_TPT;HOTEL_WWW_TPT;TEST_TPT;BENCH_TPT;HOTEL_RESERVE_PPROF_MUTEX;CACHESRV_PPROF_MUTEX;"
  $COMMAND
ENDSSH
