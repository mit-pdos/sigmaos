#!/bin/sh

DIR=$(dirname $0)
. $DIR/env/env.sh

$DIR/umount.sh
pgrep -x bootkernel > /dev/null && killall bootkernel

sudo ip link del `ip link list | grep -o "sb[a-z]*" | head -1`
sudo ip link del `ip link list | grep -o "sp[0-9]+" | head -1`
sudo iptables -S | ./delroute.sh
