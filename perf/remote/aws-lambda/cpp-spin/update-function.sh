#!/bin/bash

cd build
make
make aws-lambda-package-api
aws lambda update-function-code --function-name cpp-spin --zip-file fileb://api.zip
