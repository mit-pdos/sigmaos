#!/bin/sh

DIR=$(dirname $0)
. $DIR/env/env.sh

$DIR/umount.sh
for d in "realm" "kernel" "user"; do
  for p in `ls "cmd/$d"`; do
    pgrep -x $p > /dev/null && killall $p
  done
done

sudo ip link del `ip link list | grep -o "sb[a-z]*" | head -1`
sudo ip link del `ip link list | grep -o "sp[0-9]+" | head -1`
sudo iptables -S | ./delroute.sh
