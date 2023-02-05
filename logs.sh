#!/bin/bash

for containerid in $(docker ps --format "{{.Names}}"); do
    if [[ $containerid == sigma-* ]] ; then
        echo "========== Logs for $containerid =========="
        docker logs $containerid
    fi
done
