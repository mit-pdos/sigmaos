#!/bin/bash

usage() {
  echo "Usage: $0"
}

if [ $# -ne 0 ]; then
    usage
    exit 1
fi

IMAGE="sigmaos"

go clean -testcache

#
# test some support package
#

for T in path serr linuxsched per sigmap memfs; do
    go test -v sigmaos/$T
done


#
# test with a kernel with just named
#

for T in reader writer stats; do
    go test -v sigmaos/$T
done

exit 0
    
go test $@ sigmaos/reader
go test $@ sigmaos/writer
go test $@ sigmaos/stats
go test $@ sigmaos/fslib
go test $@ sigmaos/semclnt
go test $@ sigmaos/electclnt

#
# test proxy
#

go test $@ sigmaos/proxy

#
# tests a full kernel (one "fake" realm)
#

go test $@ sigmaos/procclnt

go test $@ sigmaos/ux
go test -v sigmaos/fslib -path "name/ux/~local/fslibtest/" -run ReadPerf

go test $@ sigmaos/s3
go test -v sigmaos/fslib -path "name/s3/~local/9ps3/fslibtest/" -run ReadPerf

go test $@ sigmaos/bootclnt
go test $@ sigmaos/leaderclnt
go test $@ sigmaos/leadertest
go test $@ sigmaos/snapshot

go test $@ sigmaos/group
go test $@ sigmaos/sessclnt

go test $@ sigmaos/cacheclnt

# dbd_test and wwwd_test requires mariadb running
pgrep mariadb >/dev/null && go test $@ sigmaos/www

go test $@ sigmaos/mr
go test $@ sigmaos/kv
go test $@ sigmaos/hotel

# XXX broken
# go test $@ sigmaos/cmd/user/test2pc
# go test $@ sigmaos/cmd/user/test2pc2

#
# test with realms
#

go test $@ sigmaos/realm

# run without realm?
# XXX needs fixing
# go test $@ -timeout=45m sigmaos/replica
