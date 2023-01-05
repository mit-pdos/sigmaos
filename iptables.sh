#!/bin/bash

# bail out if the sigmab bridge already exists
sudo iptables -nvL FORWARD | grep -c sigmab > /dev/null && exit 1

# enable ip forwarding
echo 1 | sudo tee /proc/sys/net/ipv4/ip_forward

iface=$(route | grep "default" | tr -s ' ' | cut -d ' ' -f8)

# do this once; routing rules for bridge are inserted by scnet
sudo iptables --append POSTROUTING --table nat --out-interface $iface --jump MASQUERADE
