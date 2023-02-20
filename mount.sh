#!/bin/sh

#
# Start a proxy for the named in container with IP address <IPaddr>
# and mount that named at /mnt/9p.
#

usage() {
  echo "Usage: $0 <IPaddr>"  1>&2
}

if [ $# -ne 1 ]; then
    usage
    exit 1
fi

./bin/linux/proxyd $1 $1:1111 &

sleep 1

sudo mount -t 9p -o trans=tcp,aname=`whoami`,uname=`whoami`,port=1110 $1 /mnt/9p
