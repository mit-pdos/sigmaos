#!/bin/sh

#
# Start a rootrealm named and a proxy in container with IP address
# <IPaddr> and mount the named at /mnt/9p.
#

usage() {
  echo "Usage: $0 <IPaddr>"  1>&2
}

if [ $# -ne 1 ]; then
    usage
    exit 1
fi

./start-kernel.sh --boot all sigma-named

./bin/linux/proxyd $1 &

sleep 1

sudo mount -t 9p -o trans=tcp,aname=`whoami`,uname=`whoami`,port=1110 $1 /mnt/9p
