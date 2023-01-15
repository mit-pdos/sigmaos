#!/bin/bash

docker build -t sigmaos .
docker build -f Dockeruser -t sigmauser .
