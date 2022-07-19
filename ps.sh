#!/bin/bash

if [ $# -eq 0 ]
then
      echo "Usage $0 <REALM>"
      exit 1
fi


find /mnt/9p/realm-nameds/$1/procd/*/running/ -type f -print | xargs -I {} jq -rc '.Program|.Args' {}
