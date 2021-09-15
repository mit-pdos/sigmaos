#!/bin/bash

./start.sh 

./bin/user/memfs-raft-replica pid1 1 localhost:30001,localhost:30002,localhost:30003 name/raft name/raft-replica &
./bin/user/memfs-raft-replica pid2 2 localhost:30001,localhost:30002,localhost:30003 name/raft name/raft-replica &
./bin/user/memfs-raft-replica pid3 3 localhost:30001,localhost:30002,localhost:30003 name/raft name/raft-replica &
