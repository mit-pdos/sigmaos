#!/bin/bash

#
# Install the setuid scnet to make ethernet bridge when running a
# sigmaos kernel.
#

DIR=$(dirname $0)
. $DIR/env/env.sh

SNET="bin/linux/scnet"
sudo chown root:root $SNET
sudo chmod u+s $SNET
sudo mv $SNET /usr/bin/scnet
