#!/bin/bash

export NAMED=:1111

go clean -testcache

#
# test some support package
#

go test $1 ulambda/linuxsched

#
# tests without servers
#
go test $1 ulambda/ninep
go test $1 ulambda/memfs
go test $1 ulambda/fsclnt

#
# test with just named
#
go test $1 ulambda/reader
go test $1 ulambda/writer
go test $1 ulambda/stats
go test $1 ulambda/fslib
go test $1 ulambda/fenceclnt
go test $1 ulambda/semclnt

#
# test proxy
#

./proxy/test.sh

#
# tests kernel (without realms)
#

go test $1 ulambda/ux
go test $1 ulambda/s3
go test $1 ulambda/kernel
go test $1 ulambda/procclnt
# run this procclnt test again from a script to grep the output from
# fencers for correctness
./fences/test.sh

go test $1 ulambda/group
go test $1 ulambda/kv
go test $1 ulambda/mr

# dbd_test and wwwd_test requires mariadb running
pgrep mariadb >/dev/null && go test $1 ulambda/dbd
pgrep mariadb >/dev/null && go test $1 ulambda/cmd/user/wwwd

# XXX broken
# go test $1 ulambda/cmd/user/test2pc
# go test $1 ulambda/cmd/user/test2pc2

#
# test with realms
#

go test $1 ulambda/realm

# run without realm?
# XXX needs fixing
# go test $1 -timeout=45m ulambda/replica
 
