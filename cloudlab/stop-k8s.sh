#!/bin/bash

usage() {
  echo "Usage: $0 [--parallel]" 1>&2
}

PARALLEL=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --vpc)
    shift
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

DIR=$(dirname $0)
source $DIR/env.sh

vms=`cat servers.txt | cut -d " " -f2` 

vma=($vms)
MAIN="${vma[0]}"

# Log into the control-plane node to drain all pods from other nodes.
ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$MAIN <<"ENDSSH"
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
  ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm <<ENDSSH
    yes | sudo kubeadm reset
ENDSSH
done
