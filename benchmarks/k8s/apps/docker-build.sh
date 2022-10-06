#!/bin/bash

app=$(basename $(pwd))

rm ./bin/*

cd ..
docker build -t arielszekely/$app --build-arg APP_NAME=$app -f Dockerfile .
docker push arielszekely/$app
