#!/bin/bash

app=$(basename $(pwd))

cd ..
docker build -t arielszekely/$app -f $app/Dockerfile .
docker push arielszekely/$app
