#!/bin/bash

/opt/wasi-sdk/bin/clang++ \
-O2 \
--sysroot=/opt/wasi-sdk/share/wasi-sysroot \
-Wall \
-o hello.wasm \
hello.cpp
