#!/bin/bash

ROOT_DIR=$(realpath $(dirname $0)/..)

rm -rf /tmp/gutenberg
mkdir /tmp/gutenberg
cp $ROOT_DIR/input/* /tmp/gutenberg
rm /tmp/gutenberg/gutenberg.txt

for f in `ls /tmp/gutenberg/*` ; do 
  echo "copy $f to S3 buckets"
  aws s3 cp $f s3://9ps3/gutenberg/$(basename $f)
  aws s3 cp $f s3://mr-restricted/gutenberg/$(basename $f)
done 
