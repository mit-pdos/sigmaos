#!/bin/bash

usage() {
  echo "Usage: $0 [--nosort]" 1>&2
}

SORT="sort"
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --nosort)
    shift
    SORT=""
    ;;
   *)
   echo "unexpected argument $1"
   usage
   exit 1
 esac
done

for containerid in $(docker ps -a --format "{{.Names}}"); do
    if [[ $containerid == sigma-* ]] ; then
        echo "========== Logs for $containerid =========="
        if [ $SORT == "sort" ]; then 
            docker logs $containerid | sort -k 1
        else 
            docker logs $containerid
        fi
    fi
done
