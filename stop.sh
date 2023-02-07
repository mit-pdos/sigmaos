#!/bin/sh

usage() {
  echo "Usage: $0 [--parallel]" 1>&2
}

PARALLEL=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
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



if mount | grep -q 9p; then
    echo "umount /mnt/9p"
    ./umount.sh
fi

pgrep -x proxyd > /dev/null && killall -9 proxyd

if docker ps -a | grep -qE 'sigma|uprocd|bootkerne'; then
  for container in $(docker ps -a | grep -E 'sigma|uprocd|bootkerne' | cut -d ' ' -f1) ; do
    stop="
      docker stop $container 
      docker rm $container
    "
    if [ -z "$PARALLEL" ]; then
      eval "$stop"
    else
      (
        eval "$stop"
      ) &
    fi
  done
fi

wait
