#!/bin/bash

if docker ps -a | grep -qE 'sigma|uprocd|bootkerne'; then
  for containerid in $(docker ps -a | grep -E 'sigma|uprocd|bootkerne' | cut -d ' ' -f1); do
    imageid=$(docker ps -a | grep $containerid | tr -s " " | cut -d ' ' -f2)
    if [[ $imageid == "mariadb" ]]; then
      continue
    fi
    echo "========== Logs for image $imageid =========="
    docker logs $containerid
  done
fi
