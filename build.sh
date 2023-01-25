#!/bin/bash

# decrypt the aws and docker secrets.
SECRETS="aws/.aws/credentials"
for F in $SECRETS
do
  yes | gpg --output $F --decrypt ${F}.gpg || exit 1
done

./make.sh --norace --parallel linux
DOCKER_BUILDKIT=1 docker build -t sigmaosbase .
docker build -f Dockerkernel -t sigmaos .

rm $SECRETS

docker build -f Dockeruser -t sigmauser .
