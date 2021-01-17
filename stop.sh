#!/bin/sh

killall named
killall proxyd
killall ulambd
sudo umount /mnt/9p
