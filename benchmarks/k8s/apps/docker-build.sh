#!/bin/bash

app=$(basename $(pwd))

cd ..
docker build -t arielszekely/$app --build-arg APP_NAME=$app -f Dockerfile .
docker push arielszekely/$app
