#!/bin/sh

killall named
killall proxyd
killall schedd
killall kvd
sudo umount /mnt/9p
