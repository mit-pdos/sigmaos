#!/bin/bash

#
# Install aws credentials for sigma's s3 service
#

DIR=$(dirname $0)
. $DIR/env/env.sh

# install aws credentials
mkdir -p $SIGMAHOME/.aws
SECRETS="aws/.aws/credentials"
for f in $SECRETS
do
  yes | gpg --output $f --decrypt ${f}.gpg || exit 1
done
cp aws/.aws/fk-credentials $SIGMAHOME/.aws/credentials 
chmod 600 $SIGMAHOME/.aws/credentials
rm $SECRETS
