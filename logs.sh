#!/bin/bash

for containerid in $(docker ps -a --format "{{.Names}}"); do
    if [[ $containerid == sigma-* ]] ; then
        echo "========== Logs for $containerid =========="
        docker logs $containerid | sort -k 1
    fi
done
