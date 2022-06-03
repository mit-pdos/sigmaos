#!/bin/sh

./umount.sh
for d in "realm" "kernel" "user"; do
  for p in `ls bin/$d`; do
    pgrep -x $p > /dev/null && killall $p
  done
done
