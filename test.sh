#!/bin/bash

usage() {
  echo "Usage: $0"
}

go clean -testcache

#
# test some support package
#

for T in path intervals serr linuxsched perf sigmap; do
    go test $@ sigmaos/$T
done

#
# test proxy with just named
#

go test $@ sigmaos/proxy -start

#
# test with a kernel with just named
#

for T in reader writer stats fslib semclnt electclnt; do
    go test $@ sigmaos/$T -start
done

# go test $@ sigmaos/memfs -start   # no pipes
# go test $@ sigmaos/fslibsrv -start  # no perf

# test memfs using schedd's memfs
go test $@ sigmaos/fslib -start -path "name/schedd/~local/" 

#
# tests a full kernel using root realm
#

for T in named procclnt ux s3 bootkernelclnt leaderclnt leadertest group sessclnt cacheclnt www; do
    go test $@ sigmaos/$T -start
done


go test $@ sigmaos/fslibsrv -start -path "name/ux/~local/" -run ReadPerf
go test $@ sigmaos/fslibsrv -start -path "name/s3/~local/9ps3/" -run ReadPerf

#
# applications
#

for T in imgresized mr; do
    go test $@ sigmaos/$T -start
done

#
# run only the most stringent test with kv
#
go test $@ sigmaos/kv -start -run Fail

#
# application with several kernels and db
#

go test $@ sigmaos/hotel -start

#
# test with realms
#

go test $@ sigmaos/realmclnt -start

#
# Container tests (will OOM your machine if you don't have 1:1 memory:swap ratio
#
go test $@ sigmaos/container -start

#
# tests with overlays
#

go test $@ sigmaos/procclnt -start --overlays --run TestWaitExitSimpleSingle
go test $@ sigmaos/cacheclnt -start --overlays --run TestCacheClerk
go test $@ sigmaos/hotel -start --overlays --run GeoSingle
go test $@ sigmaos/hotel -start --overlays --run Www
go test $@ sigmaos/realmclnt -start --overlays --run Basic
go test $@ sigmaos/realmclnt -start --overlays --run WaitExitSimpleSingle
go test $@ sigmaos/realmclnt -start --overlays --run RealmNetIsolation
