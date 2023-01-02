#!/bin/bash

DIR=$(dirname $0)
. $DIR/env/env.sh

VERSION=$(cat "${VERSION_FILE}")

./install.sh --realm testrealm # --from s3

for ND in :1111 :1111,:1112,:1113
do
  export SIGMANAMED=$ND
  echo "============ RUN SIGMANAMED=$ND"
  go clean -testcache
  
  #
  # test some support package
  #

  go test $@ sigmaos/path
  go test $@ sigmaos/serr
  go test $@ sigmaos/linuxsched
  go test $@ sigmaos/perf
  
  #
  # tests without kernel
  #
  go test $@ sigmaos/sigmap
  go test $@ sigmaos/memfs
  
  #
  # test with a kernel with just named
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
  # tests a full kernel (one "fake" realm)
  #
  
  go test $@ sigmaos/procclnt --version=$VERSION

  go test $@ sigmaos/ux --version=$VERSION
  go test -v sigmaos/fslib --version=$VERSION -path "name/ux/~local/fslibtest/" -run ReadPerf
  
  go test $@ sigmaos/s3 --version=$VERSION
  go test -v sigmaos/fslib --version=$VERSION -path "name/s3/~local/9ps3/fslibtest/" -run ReadPerf
  
  go test $@ sigmaos/bootclnt --version=$VERSION
  go test $@ sigmaos/leaderclnt --version=$VERSION
  go test $@ sigmaos/leadertest --version=$VERSION
  go test $@ sigmaos/snapshot --version=$VERSION
  
  go test $@ sigmaos/group --version=$VERSION
  go test $@ sigmaos/sessclnt --version=$VERSION

  go test $@ sigmaos/shardsvcmgr --version=$bVERSION
  go test $@ sigmaos/cacheclnt --version=$VERSION

  # dbd_test and wwwd_test requires mariadb running
  pgrep mariadb >/dev/null && go test $@ sigmaos/www
  
  go test $@ sigmaos/mr --version=$VERSION
  go test $@ sigmaos/kv --version=$VERSION
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
