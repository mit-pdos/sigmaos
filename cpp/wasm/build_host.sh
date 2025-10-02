#!/bin/bash

clang++ host.cpp \
-I/home/nour/.wasmedge/include \
-L/home/nour/.wasmedge/lib \
-Wl,-rpath,/home/nour/.wasmedge/lib \
-lwasmedge \
-o host