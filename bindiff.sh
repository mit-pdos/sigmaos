#!/bin/bash

objdump -h bin/user/sleeper-v1.0 > /tmp/sleeper-headers
objdump -h bin/user/spinner-v1.0 > /tmp/spinner-headers
objdump -d bin/user/sleeper-v1.0 > /tmp/sleeper
objdump -d bin/user/spinner-v1.0 > /tmp/spinner
objdump -d --no-addresses bin/user/sleeper-v1.0 > /tmp/sleeper-2
objdump -d --no-addresses bin/user/spinner-v1.0 > /tmp/spinner-2
diff /tmp/sleeper-2 /tmp/spinner-2 > /tmp/diff-2.txt
