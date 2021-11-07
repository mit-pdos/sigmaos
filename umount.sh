#!/bin/sh

pgrep proxyd > /dev/null && killall proxyd
sudo umount /mnt/9p
