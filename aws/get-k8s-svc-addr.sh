#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC --svc SVC_NAME" 1>&2
}

VPC=""
SVC_NAME=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --vpc)
    shift
    VPC=$1
    shift
    ;;
  --svc)
    shift
    SVC_NAME=$1
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

if [ -z "$VPC" ] || [ -z "$SVC_NAME" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

vms=`./lsvpc.py $VPC | grep -w VMInstance | cut -d " " -f 5`

vma=($vms)
MAIN="${vma[0]}"

ssh -i key-$VPC.pem ubuntu@$MAIN /bin/bash <<ENDSSH
  kubectl get svc --all-namespaces | grep -wE "^$SVC_NAME" | tr -s " " | cut -d " " -f 3
ENDSSH
