#!/bin/sh

DIR=$(dirname $0)
. $DIR/env/env.sh

if mount | grep 9p; then
    echo "umount /mnt/9p"
    $DIR/umount.sh
fi

pgrep -x bootkernel > /dev/null && killall bootkernel
pgrep -x bootsys > /dev/null && killall bootsys

b=$(ip link list | grep -o "[0-9]*: sb[a-z]*")
if [ ! -z "$b" ]; then
   b=$(echo $b | cut -f 2 -d ' ')
   echo "delete bridge $b"
   sudo ip link del $b
fi
b=$(ip link list | grep -o "[0-9]*: sp[0-9]+]*")
if [ ! -z "$b" ]; then
   b=$(echo $b | cut -f 2 -d ' ')
   echo "delete veth $b"
   sudo ip link del $b
fi

sudo iptables -S | ./delroute.sh
