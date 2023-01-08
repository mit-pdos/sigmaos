#!/bin/bash

while read -r line; do
    if [ ! -z "$line" ]; then
	sudo iptables -D $line
    fi
done <<< $(grep "sb" $1 | cut -d' ' -f2-)
