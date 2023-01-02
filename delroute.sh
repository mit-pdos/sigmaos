#!/bin/bash

while read -r line; do
    sudo iptables -D $line
done <<< $(grep "sb" $1 | cut -d' ' -f2-)
