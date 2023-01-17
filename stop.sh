#!/bin/sh

if mount | grep -q 9p; then
    echo "umount /mnt/9p"
    $DIR/umount.sh
fi

pgrep -x proxyd > /dev/null && killall proxyd
