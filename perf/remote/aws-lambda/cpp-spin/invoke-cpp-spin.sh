#!/bin/bash

#aws apigateway test-invoke-method --

curl -v -X POST \
  'https://m5ica91644.execute-api.us-east-1.amazonaws.com/default/cpp-spin?dim=64&its=5000' \
  -H 'content-type: application/json' \
  -d '{ "baseline": false }'
