#!/bin/bash

DIR=$(dirname $0)
. $DIR/.env

# Copy to S3
aws s3 cp --recursive bin s3://9ps3/bin
