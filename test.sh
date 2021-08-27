#!/bin/bash

export NAMED=127.0.0.1:1111

go test $1 ulambda/ninep
go test $1 ulambda/memfs
go test $1 ulambda/fsclnt
go test $1 ulambda/fslib
go test $1 ulambda/sync
go test $1 ulambda/stats
(cd fss3; go test $1)
go test $1 ulambda/proc
go test $1 ulambda/depproc
go test $1 ulambda/idemproc
(cd kv; go test $1)
./test-mr.sh
./test-kv.sh
(cd replica; go test -timeout=45m $1)
