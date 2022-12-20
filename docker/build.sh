#!/bin/bash

cp ~/.ssh/id_rsa.pub .
docker build --platform linux/x86_64 -t sigmaos -f docker/Dockerfile .
rm id_rsa.pub
