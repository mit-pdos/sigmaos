#!/bin/bash

cd ../
docker build -t arielszekely/spin -f docker/Dockerfile .
docker push arielszekely/spin
