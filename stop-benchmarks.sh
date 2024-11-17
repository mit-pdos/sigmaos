#!/bin/bash

kill -9 $(pgrep -f "benchmarks.test")
kill -9 $(pgrep -f "start-kernel.sh")
