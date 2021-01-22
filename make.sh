#!/bin/sh

for f in `ls cmd`
do
    echo "Build $f"
    go build -race -o bin/$f cmd/$f/main.go
done

