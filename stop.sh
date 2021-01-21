#!/bin/sh

killall named
killall proxyd
killall schedd
sudo umount /mnt/9p
