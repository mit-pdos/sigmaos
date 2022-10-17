#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC [--parallel]" 1>&2
}

VPC=""
PARALLEL=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --vpc)
    shift
    VPC=$1
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

DIR=$(dirname $0)
. $DIR/../.env

vms=`./lsvpc.py $VPC | grep -w VMInstance | cut -d " " -f 5`

vma=($vms)
MAIN="${vma[0]}"

# Log into the control-plane node to drain all pods from other nodes.
ssh -i key-$VPC.pem ubuntu@$MAIN /bin/bash <<"ENDSSH"
  lines=$(kubectl get nodes | tail -n +2)
  while IFS= read -r line; do
    name=$(echo $line | cut -d " " -f1)
    kubectl drain $name --delete-emptydir-data --force --ignore-daemonsets
    kubectl delete node $name
  done <<< "$lines"
ENDSSH

# Reverse the list of servers. We want to kill the control-plane node last.
vms=$(echo $vms | tr ' ' '\n' | tac | tr '\n' ' ')

for vm in $vms
do
  echo "STOP $vm"
  ssh -i key-$VPC.pem ubuntu@$vm /bin/bash <<ENDSSH
    yes | sudo kubeadm reset
ENDSSH
done
