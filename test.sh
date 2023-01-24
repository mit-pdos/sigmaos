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

for T in reader writer stats reader writer stats fslib semclnt electclnt; do
    go test -v sigmaos/$T
done

#
# test proxy
#

go test -v sigmaos/proxy

#
# tests a full kernel using root realm
#

# procclnt; two tests fail:
# --- FAIL: TestSpawnProcdCrash (0.00s)
# --- FAIL: TestMaintainReplicationLevelCrashProcd (0.00s)
# sessclnt; TestWriteCrash fails

for T in procclnt ux s3 bootkernelclnt leaderclnt leadertest snapshot group sessclnt cacheclnt; do
    go test -v sigmaos/$T
done
    
go test -v sigmaos/fslib -path "name/ux/~local/fslibtest/" -run ReadPerf
go test -v sigmaos/fslib -path "name/s3/~local/9ps3/fslibtest/" -run ReadPerf

exit 0

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
