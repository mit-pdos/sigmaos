#!/bin/sh

if mount | grep -q 9p; then
    echo "umount /mnt/9p"
    ./umount.sh
fi

pgrep -x proxyd > /dev/null && killall -9 proxyd

if docker ps -a | grep -qE 'sigma|uprocd|bootkerne'; then
    docker stop $(docker ps -a | grep -E 'sigma|uprocd|bootkerne' | cut -d ' ' -f1)
    docker rm $(docker ps -a | grep -E 'sigma|uprocd|bootkerne' | cut -d ' ' -f1)
fi
