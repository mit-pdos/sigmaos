#!/bin/sh

DIR=$(dirname $0)
. $DIR/env/env.sh

if mount | grep -q 9p; then
    echo "umount /mnt/9p"
    $DIR/umount.sh
fi

pgrep -x bootkernel > /dev/null && killall bootkernel
pgrep -x bootsys > /dev/null && killall bootsys
pgrep -x proxyd > /dev/null && killall proxyd

while read -r line; do
    if [ ! -z "$line" ]; then
        b=$(echo $line | cut -f 2 -d ' ')
        echo "delete bridge $b"
        sudo ip link del $b
    fi
done <<< $(ip link list | grep -o "[0-9]*: sb[a-z]*")

while read -r line; do
    if [ ! -z "$line" ]; then
        links=$(echo $line | cut -f 2 -d ' ')
        l1=$(echo $links | cut -f 1 -d '@')
        l2=$(echo $links | cut -f 2 -d '@')
        echo "delete ifaces $l1 $l2"
        sudo ip link del $l1
        sudo ip link del $l2
    fi
done <<< $(ip link list | grep -o "[0-9]*: sp[0-9]*@sp[0-9]*")

sudo iptables -S | ./delroute.sh
