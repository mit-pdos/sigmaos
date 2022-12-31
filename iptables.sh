#!/bin/bash

sudo iptables -nvL FORWARD | grep -c sigmab > /dev/null && exit 1

echo 1 | sudo tee /proc/sys/net/ipv4/ip_forward

sudo iptables --append FORWARD --in-interface sigmab --out-interface sigmab --jump ACCEPT
sudo iptables --append FORWARD --in-interface wlp2s0 --out-interface sigmab --jump ACCEPT
sudo iptables --append FORWARD --in-interface sigmab --out-interface wlp2s0 --jump ACCEPT
sudo iptables --append POSTROUTING --table nat --out-interface wlp2s0 --jump MASQUERADE
