#!/bin/bash

app=$(basename $(pwd))

docker build -t arielszekely/$app -f Dockerfile .
docker push arielszekely/$app
