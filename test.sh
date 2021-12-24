#!/bin/bash

export NAMED=:1111

go clean -testcache

#
# tests without servers
#
go test $1 ulambda/ninep
go test $1 ulambda/memfs
go test $1 ulambda/fsclnt

#
# test with just named
#
go test $1 ulambda/fslib
go test $1 ulambda/stats
go test $1 ulambda/sync

#
# test proxy
#

./proxy/test.sh

#
# tests kernel (without realms)
#

go test $1 ulambda/kernel
go test $1 ulambda/ux
go test $1 ulambda/s3
go test $1 ulambda/procclnt
go test $1 ulambda/kv
go test $1 ulambda/mr

# wwwd_test requires mariadb running
pgrep mariadb >/dev/null && go test $1 ulambda/cmd/user/wwwd

go test $1 ulambda/cmd/user/test2pc
go test $1 ulambda/cmd/user/test2pc2


#
# test with realms
#

go test $1 ulambda/realm # Fails due to dropped eviction signals

# run without realm?
# XXX needs fixing
# go test $1 -timeout=45m ulambda/replica
 
