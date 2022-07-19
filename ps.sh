#!/bin/bash

if [ $# -eq 0 ]
then
      echo "Usage $0 <REALM>"
      exit 1
fi

for f in /mnt/9p/realm-nameds/$1/procd/*:*
do
        echo "===" $f
        find "$f/running/" -type f -print | xargs -I {} jq -rc '.Program,.Args' {}
done
