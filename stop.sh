#!/bin/sh

if mount | grep -q 9p; then
    echo "umount /mnt/9p"
    $DIR/umount.sh
fi

pgrep -x proxyd > /dev/null && killall -9 proxyd

docker stop $(docker ps -a | grep 'sigma' | cut -d ' ' -f1)
docker rm $(docker ps -a | grep 'sigma' | cut -d ' ' -f1)
