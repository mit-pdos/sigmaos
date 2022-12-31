#!/bin/bash

#
# Install the sigmaOS software into root file system, either from the
# local build or from s3.
#

usage() {
    echo "Usage: $0 --realm REALM [--from FROM] [--profile PROFILE] [--version VERSION]" 1>&2
}

FROM="local"
REALM=""
VERSION=""
PROFILE=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --from)
    shift
    FROM="$1"
    shift
    ;;
  --realm)
    shift
    REALM=$1
    shift
    ;;
  --version)
    shift
    VERSION=$1
    shift
    ;;
  --profile)
    shift
    PROFILE="--profile $1"
    shift
    ;;
  -help)
    usage
    exit 0
    ;;
  *)
    echo "unexpected argument $1"
    usage
    exit 1
    ;;
  esac
done

if [ -z "$REALM" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

DIR=$(dirname $0)
. $DIR/env/env.sh

echo "install sigmaos in $SIGMAHOME"

mkdir -p $PRIVILEGED_BIN
rm -rf $PRIVILEGED_BIN/*
rm -rf $SIGMAHOME/$REALM/bin/user/*
if [ $FROM == "local" ]; then
  if [ -z "$VERSION" ]; then
    VERSION=$(cat "${VERSION_FILE}")
  fi
  # Make the user program dir
  mkdir -p $SIGMAHOME/$REALM/bin/user/$VERSION/
  # Copy from local
  cp -r bin/user/* $SIGMAHOME/$REALM/bin/user/$VERSION/
  cp -r bin/realm $PRIVILEGED_BIN
  cp -r bin/kernel $PRIVILEGED_BIN
  cp -r bin/linux $PRIVILEGED_BIN
elif [ $FROM == "s3" ]; then
  # Copy kernel & realm dirs from s3
  aws s3 cp --recursive s3://$REALM/bin/realm $PRIVILEGED_BIN/realm $PROFILE
  aws s3 cp --recursive s3://$REALM/bin/kernel $PRIVILEGED_BIN/kernel $PROFILE
  aws s3 cp --recursive s3://$REALM/bin/kernel $PRIVILEGED_BIN/linux $PROFILE
  chmod --recursive +x $PRIVILEGED_BIN
else
  echo "Unrecognized bin source: $FROM"
  exit 1
fi

cp bootclnt/boot*.yml $SIGMAHOME/$REALM/
cp seccomp/whitelist.yml $SIGMAHOME/

for d in etc dev sys proc usr lib lib64
do        
    mkdir -p $SIGMAHOME/$REALM/$d
done
for f in urandom null
do
    echo -n > $SIGMAHOME/$REALM/dev/$f
done
