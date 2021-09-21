#!/bin/bash

./start.sh 

./bin/user/memfs-raft-replica pid1 1 localhost:30001 name/raft name/raft-replica &
sleep 1
./bin/user/memfs-raft-replica pid2 2 localhost:30001,localhost:30002 name/raft name/raft-replica &
sleep 1
./bin/user/memfs-raft-replica pid3 3 localhost:30001,localhost:30002,localhost:30003 name/raft name/raft-replica &
