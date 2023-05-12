#!/bin/bash

usage() {
  echo "Usage: $0"
}

go clean -testcache

#
# test some support package
#

for T in path intervals serr linuxsched perf sigmap memfs; do
    go test $@ sigmaos/$T
done

#
# test with a kernel with just named
#

for T in reader writer stats fslib semclnt electclnt; do
    go test $@ sigmaos/$T -start
done

#
# test proxy
#

go test $@ sigmaos/proxy -start

#
# tests a full kernel using root realm
#

for T in procclnt ux s3 bootkernelclnt leaderclnt leadertest snapshot group sessclnt cacheclnt www; do
    go test $@ sigmaos/$T -start
done
    
go test $@ sigmaos/fslib -start -path "name/ux/~local/fslibtest/" -run ReadPerf
go test $@ sigmaos/fslib -start -path "name/s3/~local/9ps3/fslibtest/" -run ReadPerf

#
# applications
#

for T in imgresized mr kv; do
    go test $@ sigmaos/$T -start
done

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
