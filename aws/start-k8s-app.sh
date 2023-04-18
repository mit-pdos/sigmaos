#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC --path APP_PATH --nrunning N_RUNNING" 1>&2
}

VPC=""
APP_PATH=""
N_RUNNING=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --vpc)
    shift
    VPC=$1
    shift
    ;;
  --path)
    shift
    APP_PATH=$1
    shift
    ;;
  --nrunning)
    shift
    N_RUNNING=$1
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

if [ -z "$VPC" ] || [ -z "$APP_PATH" ] || [ -z "$N_RUNNING" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

vms=`./lsvpc.py $VPC | grep -w VMInstance | cut -d " " -f 5`
vms_privaddr=`./lsvpc.py $VPC --privaddr | grep -w VMInstance | cut -d " " -f 6`

vma=($vms)
vma_privaddr=($vms_privaddr)
MAIN="${vma[0]}"
MAIN_PRIVADDR="${vma_privaddr[0]}"

export APP_PATH=$APP_PATH
export N_RUNNING=$N_RUNNING

CMD=$(
envsubst '$APP_PATH:$N_RUNNING' <<'ENDSSH'
  kubectl apply -Rf $APP_PATH > /dev/null 2>&1
  until [ $(kubectl get pods | grep -w "Running" | wc -l ) == "$N_RUNNING" ]; do
    echo "Missing pods" > /dev/null 2>&1
    sleep 2s
  done
  echo "$N_RUNNING pods running"
ENDSSH
)

ssh -i key-$VPC.pem ubuntu@$MAIN /bin/bash <<ENDSSH
  $CMD
ENDSSH
