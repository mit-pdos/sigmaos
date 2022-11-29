#!/bin/bash

DIR=$(dirname $0)
. $DIR/.env

VERSION=$(cat "${VERSION_FILE}")

./install.sh --realm test-realm # --from s3

for ND in :1111 :1111,:1112,:1113
do
  export NAMED=$ND
  echo "============ RUN NAMED=$ND"
  go clean -testcache
  
  #
  # test some support package
  #

  go test $@ sigmaos/spcodec
  go test $@ sigmaos/linuxsched
  go test $@ sigmaos/perf
  
  #
  # tests without servers
  #
  go test $@ sigmaos/ninep
  go test $@ sigmaos/memfs
  go test $@ sigmaos/pathclnt
  
  #
  # test with just named
  #
  go test $@ sigmaos/reader --version=$VERSION
  go test $@ sigmaos/writer --version=$VERSION
  go test $@ sigmaos/stats --version=$VERSION
  go test $@ sigmaos/fslib --version=$VERSION
  go test $@ sigmaos/semclnt --version=$VERSION
  go test $@ sigmaos/electclnt --version=$VERSION
  
  #
  # test proxy
  #
  
  go test $@ sigmaos/proxy --version=$VERSION
  
  #
  # tests kernel (with 1 "fake" realm)
  #
  
  go test $@ sigmaos/procclnt --version=$VERSION

  go test $@ sigmaos/ux --version=$VERSION
  go test -v sigmaos/fslib --version=$VERSION -path "name/ux/~ip/fslibtest/" -run ReadPerf
  
  go test $@ sigmaos/s3 --version=$VERSION
  go test -v sigmaos/fslib --version=$VERSION -path "name/s3/~ip/9ps3/fslibtest/" -run ReadPerf
  
  go test $@ sigmaos/kernel --version=$VERSION
  go test $@ sigmaos/leaderclnt --version=$VERSION
  go test $@ sigmaos/leadertest --version=$VERSION
  go test $@ sigmaos/snapshot --version=$VERSION
  
  go test $@ sigmaos/group --version=$VERSION
  go test $@ sigmaos/sessclnt --version=$VERSION
  
  # dbd_test and wwwd_test requires mariadb running
  pgrep mariadb >/dev/null && go test $@ sigmaos/www
  
  go test $@ sigmaos/mr --version=$VERSION
  go test $@ sigmaos/kv --version=$VERSION
  go test $@ sigmaos/cacheclnt --version=$VERSION
  go test $@ sigmaos/hotel --version=$VERSION
  
  # XXX broken
  # go test $@ sigmaos/cmd/user/test2pc
  # go test $@ sigmaos/cmd/user/test2pc2
  
  #
  # test with realms
  #
  
  go test $@ sigmaos/realm --version=$VERSION
  
  # run without realm?
  # XXX needs fixing
  # go test $@ -timeout=45m sigmaos/replica
done 
