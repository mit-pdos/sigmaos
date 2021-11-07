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
go test $1 ulambda/procbasev1
go test $1 ulambda/procidem
go test $1 ulambda/procdep
go test $1 ulambda/kv
go test $1 ulambda/cmd/user/mr

./test-kv.sh

# wwwd_test requires mariaddb running
pgrep mariadb >/dev/null && go test -v ulambda/cmd/user/wwwd

go test -v ulambda/test2pc

go test $1 -timeout=45m ulambda/replica
