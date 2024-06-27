#!/bin/sh

pgrep proxyd > /dev/null && killall npproxyd
sudo umount /mnt/9p
