#!/bin/bash

usage() {
  echo "Usage: $0 --path APP_PATH --nrunning N_RUNNING" 1>&2
}

APP_PATH=""
N_RUNNING=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --parallel)
    shift
    ;;
  --vpc)
    shift
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

if [ -z "$APP_PATH" ] || [ -z "$N_RUNNING" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

DIR=$(dirname $0)
source $DIR/env.sh

vms=`cat servers.txt | cut -d " " -f2` 

vma=($vms)
MAIN="${vma[0]}"

export APP_PATH=$APP_PATH
export N_RUNNING=$N_RUNNING

CMD=$(
envsubst '$APP_PATH:$N_RUNNING' <<'ENDSSH'
  kubectl apply -v 6 -Rf $APP_PATH > /tmp/k8sappinit.out 2>&1
  until [ $(kubectl get pods | grep -w "Running" | wc -l ) -ge "$N_RUNNING" ]; do
    echo "Missing pods" > /dev/null 2>&1
    sleep 2s
  done
  echo "$N_RUNNING pods running"
ENDSSH
)

ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$MAIN <<ENDSSH
  $CMD
ENDSSH
