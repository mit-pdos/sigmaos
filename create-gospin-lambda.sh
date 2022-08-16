#!/bin/bash

go build -ldflags="-X ulambda/ninep.Target=aws -X ulambda/proc.Version=AAA" $RACE -o bin/user/spin-lambda cmd/user/spin-lambda/main.go

zip -j /tmp/go-spin.zip bin/user/spin-lambda
aws lambda update-function-code --function-name go-spin --zip-file fileb:///tmp/go-spin.zip #--handler go-spin --runtime go1.x
