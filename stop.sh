#!/bin/sh

DIR=$(dirname $0)
. $DIR/env/env.sh

if mount | grep -q 9p; then
    echo "umount /mnt/9p"
    $DIR/umount.sh
fi

pgrep -x bootkernel > /dev/null && killall bootkernel
pgrep -x bootsys > /dev/null && killall bootsys

while read -r line; do
    if [ ! -z "$line" ]; then
        b=$(echo $line | cut -f 2 -d ' ')
        echo "delete bridge $b"
        sudo ip link del $b
    fi
done <<< $(ip link list | grep -o "[0-9]*: sb[a-z]*")

sudo iptables -S | ./delroute.sh
