#!/bin/bash

#
# setup/disable swap on instances
#
# to enabe: $ ./setup-swap.sh --vpc vpc-02f7e3816c4cc8e7f --n 4194304
# to disable: $ ./setup-swap.sh --vpc vpc-02f7e3816c4cc8e7f
#

usage() {
  echo "Usage: $0 --vpc VPC [--parallel] [--n SWAPMEM (KB)]" 1>&2
}

VPC=""
NSWAP=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --vpc)
    shift
    VPC=$1
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

if [ -z "$VPC" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

vms=`./lsvpc.py $VPC | grep -w VMInstance | cut -d " " -f 5`
vma=($vms)

for vm in $vms; do
  echo "Setup/disable swap $NSWAP for $vm"
  install="
    ssh -i key-$VPC.pem ubuntu@$vm /bin/bash <<ENDSSH
    sudo swapoff -a
    if [ -z "$NSWAP" ]; then
#      sudo rm -f /swapfile
      echo 'Disable swap'
    else
       # Make swap file, if it hasn't been made already.
       if [ ! -f /swapfile ]; then
         sudo dd if=/dev/zero of=/swapfile bs=1024 count=$NSWAP
#         sudo fallocate -l $NSWAP /swapfile
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
