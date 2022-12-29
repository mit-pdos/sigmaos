#!/bin/bash

DIR=$(dirname $0)
. $DIR/env/env.sh

export SIGMAROOTFS=$SIGMAROOTFS
PATH=$PATH:$SIGMAHOME/bin/linux/
