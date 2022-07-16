#!/bin/bash

DIR=$(dirname $0)
. $DIR/.env

VERSION=$(cat "${VERSION_FILE}")

./install.sh --from s3 --realm test-realm

for ND in :1111 :1111,:1112,:1113
do
  export NAMED=$ND
  echo "============ RUN NAMED=$ND"
  go clean -testcache
  
  #
  # test some support package
  #

  go test $@ ulambda/npcodec
  go test $@ ulambda/linuxsched
  go test $@ ulambda/perf
  
  #
  # tests without servers
  #
  go test $@ ulambda/ninep
  go test $@ ulambda/memfs
  go test $@ ulambda/pathclnt
  
  #
  # test with just named
  #
  go test $@ ulambda/reader --version=$VERSION
  go test $@ ulambda/writer --version=$VERSION
  go test $@ ulambda/stats --version=$VERSION
  go test $@ ulambda/fslib --version=$VERSION
  go test $@ ulambda/semclnt --version=$VERSION
  go test $@ ulambda/electclnt --version=$VERSION
  
  #
  # test proxy
  #
  
  go test $@ ulambda/proxy --version=$VERSION
  
  #
  # tests kernel (with 1 "fake" realm)
  #
  
  go test $@ ulambda/procclnt --version=$VERSION

  go test $@ ulambda/ux --version=$VERSION
  go test -v ulambda/fslib --version=$VERSION -path "name/ux/~ip/fslibtest/" -run ReadPerf
  
  go test $@ ulambda/s3 --version=$VERSION
  go test -v ulambda/fslib --version=$VERSION -path "name/s3/~ip/9ps3/fslibtest/" -run ReadPerf
  
  go test $@ ulambda/kernel --version=$VERSION
  go test $@ ulambda/leaderclnt --version=$VERSION
  go test $@ ulambda/leadertest --version=$VERSION
  go test $@ ulambda/snapshot --version=$VERSION
  
  go test $@ ulambda/group --version=$VERSION
  go test $@ ulambda/sessclnt --version=$VERSION
  
  # dbd_test and wwwd_test requires mariadb running
  pgrep mariadb >/dev/null && go test $@ ulambda/dbd
  pgrep mariadb >/dev/null && go test $@ ulambda/cmd/user/wwwd
  
  
  go test $@ ulambda/mr --version=$VERSION
  go test $@ ulambda/kv --version=$VERSION
  
  # XXX broken
  # go test $@ ulambda/cmd/user/test2pc
  # go test $@ ulambda/cmd/user/test2pc2
  
  #
  # test with realms
  #
  
  go test $@ ulambda/realm --version=$VERSION
  
  # run without realm?
  # XXX needs fixing
  # go test $@ -timeout=45m ulambda/replica
done 
