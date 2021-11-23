#!/bin/bash

export NAMED=127.0.0.1:1111

go clean -testcache

go test $1 ulambda/ninep
go test $1 ulambda/memfs
go test $1 ulambda/fsclnt
go test $1 ulambda/ux
go test $1 ulambda/s3
go test $1 ulambda/fslib
go test $1 ulambda/sync
go test $1 ulambda/stats
go test $1 ulambda/procclnt
go test $1 ulambda/kv
go test $1 ulambda/cmd/user/mr
go test $1 ulambda/realm # Fails due to dropped eviction signals

./test-kv.sh

# wwwd_test requires mariadb running
pgrep mariadb >/dev/null && go test $1 ulambda/cmd/user/wwwd

go test $1 ulambda/cmd/user/test2pc

# XXX needs fixing
# go test $1 -timeout=45m ulambda/replica
