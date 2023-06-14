#!/bin/bash

app=$(basename $(pwd))

rm -rf ./bin

cd ..
DOCKER_BUILDKIT=1 docker build --progress=plain -t arielszekely/appbuilder --build-arg APP_NAME=$app -f build.Dockerfile .
DOCKER_BUILDKIT=1 docker build --progress=plain -t arielszekely/${app} --build-arg APP_NAME=$app -f Dockerfile .
docker push arielszekely/$app
