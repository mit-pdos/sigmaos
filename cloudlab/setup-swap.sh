#!/bin/bash

#
# setup/disable swap on instances
#
# to enabe: $ ./setup-swap.sh --n 4194304
# to disable: $ ./setup-swap.sh
#

usage() {
  echo "Usage: $0 [--parallel] [--n SWAPMEM (KB)]" 1>&2
}

NSWAP=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --vpc)
    shift
    shift
    ;;
  --n)
    shift
    NSWAP=$1
    shift
    ;;
  --parallel)
    shift
    PARALLEL="--parallel"
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

if [ $# -gt 0 ]; then
    usage
    exit 1
fi

LOGIN="arielck"
DIR=$(dirname $0)

vms=`cat servers.txt | cut -d " " -f2` 
vma=($vms)

for vm in $vms; do
  echo "Setup/disable swap $NSWAP for $vm"
  install="
    ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm /bin/bash <<ENDSSH
    sudo swapoff /swapfile
    if [ -z "$NSWAP" ]; then
      echo 'Disable swap'
    else
       # Make swap file, if it hasn't been made already.
       if [ ! -f /swapfile ]; then
         sudo dd if=/dev/zero of=/swapfile bs=1024 count=$NSWAP
         sudo chmod 600 /swapfile
         sudo mkswap /swapfile
       fi
       echo 'Enable swap'
       sudo swapon /swapfile
    fi
ENDSSH"
  if [ -z "$PARALLEL" ]; then
    eval "$install"
  else
  (
    eval "$install"
  ) &
  fi
done
wait
