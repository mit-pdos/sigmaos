#!/bin/bash

./make.sh --norace linux
docker build -t sigmaosbase .
docker build -f Dockerkernel -t sigmaos .
docker build -f Dockeruser -t sigmauser .
