#!/bin/bash

./make.sh --norace linux
docker build -t sigmaos .
docker build -f Dockeruser -t sigmauser .
