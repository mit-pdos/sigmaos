#!/bin/bash

#
# Install aws credentials for sigma's s3 service
#

DIR=$(dirname $0)
. $DIR/env/env.sh

# install credentials
mkdir -p ~/.aws
mkdir -p ~/.docker
SECRETS="aws/.aws/credentials aws/.docker/config.json"
for f in $SECRETS
do
  yes | gpg --output $f --decrypt ${f}.gpg || exit 1
done
# Append sigmaOS AWS config and credentials to any existing AWS config and
# credentials.
cat ./aws/.aws/config >> ~/.aws/config 
cat ./aws/.aws/credentials >> ~/.aws/credentials 
chmod 600 ~/.aws/credentials
# Append docker credentials to any existing docker credentials
cat ./aws/.docker/config.json >> ~/.docker/config.json
rm $SECRETS
