#!/bin/bash

export NAMED=127.0.0.1:1111

go clean -testcache

go test $1 ulambda/ninep
go test $1 ulambda/memfs
go test $1 ulambda/fsclnt
go test $1 ulambda/fslib
go test $1 ulambda/sync
go test $1 ulambda/stats
go test $1 ulambda/fss3
go test $1 ulambda/procbase
go test $1 ulambda/procidem
go test $1 ulambda/procdep
go test $1 ulambda/kv
./test-mr.sh
./test-kv.sh
go test $1 -timeout=45m ulambda/replica
