#!/bin/sh

DIR=$(dirname $0)
. $DIR/.env

$DIR/umount.sh
for d in "realm" "kernel" "user"; do
  for p in `ls cmd/$d`; do
    echo $p
    pgrep -x $p > /dev/null && killall $p
  done
done
