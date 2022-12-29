#!/bin/bash

#
# Install the setuid scnet.
#

DIR=$(dirname $0)
. $DIR/env/env.sh

SNET="$PRIVILEGED_BIN/linux/scnet"
sudo chown root:root $SNET
sudo chmod u+s $SNET
sudo mv $SNET $SIGMAROOTFS/usr/bin/scnet
