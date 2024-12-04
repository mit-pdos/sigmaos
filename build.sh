#!/bin/bash

usage() {
  echo "Usage: $0 [--push TAG] [--target TARGET] [--version VERSION] [--userbin USERBIN] [--parallel] [--rebuildbuilder]" 1>&2
}

PARALLEL=""
REBUILD_BUILDER="false"
TAG=""
TARGET="local"
VERSION="1.0"
USERBIN="all"
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --parallel)
    shift
    PARALLEL="--parallel"
    ;;
  --rebuildbuilder)
    shift
    REBUILD_BUILDER="true"
    ;;
  --push)
    shift
    TAG="$1"
    shift
    ;;
  --target)
    shift
    TARGET="$1"
    shift
    ;;
  --version)
    shift
    VERSION="$1"
    shift
    ;;
  --userbin)
    shift
    USERBIN="$1"
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
  esac
done

if [ $# -gt 0 ]; then
    usage
    exit 1
fi

if [[ "$TAG" != "" && "$TARGET" == "local" ]] || [[ "$TAG" == "" && "$TARGET" != "local" ]] ; then
  echo "Must run with either --push set and --target=remote, or --target=local and without --push"
  exit 1
fi

TMP=/tmp/sigmaos
BUILD_LOG=/tmp/sigmaos-build
PROCD_BIN=/tmp/sigmaos-procd-bin

# tests uses hosts /tmp, which mounted in kernel container.
mkdir -p $TMP
mkdir -p $BUILD_LOG

# Make a dir to hold user proc build output
ROOT=$(pwd)
BIN=${ROOT}/bin
KERNELBIN=${BIN}/kernel
USRBIN=${BIN}/user
mkdir -p $USRBIN
# Clear the procd bin directory
rm -rf $PROCD_BIN
mkdir -p $PROCD_BIN

# build and start db container
if [ "${TARGET}" != "remote" ]; then
    ./start-network.sh
fi

# Check if a builder is running already
buildercid=$(docker ps -a | grep -w "sig-builder" | cut -d " " -f1)
rsbuildercid=$(docker ps -a | grep -w "sig-rs-builder" | cut -d " " -f1)

# Optionally stop any existing builder container, so it will be rebuilt and
# restarted.
if [[ $REBUILD_BUILDER == "true" ]]; then
  if ! [ -z "$buildercid" ]; then
    echo "========== Stopping old builder container $buildercid =========="
    docker stop $buildercid
    # Reset builder container ID
    buildercid=""
  fi
  if ! [ -z "$rsbuildercid" ]; then
    echo "========== Stopping old Rust builder container $rsbuildercid =========="
    docker stop $rsbuildercid
    # Reset builder container ID
    rsbuildercid=""
  fi
fi

if [ -z "$buildercid" ]; then
  # Build builder
  echo "========== Build builder image =========="
  DOCKER_BUILDKIT=1 docker build --progress=plain -f builder.Dockerfile -t sig-builder . 2>&1 | tee $BUILD_LOG/sig-builder.out
  echo "========== Done building builder =========="
  # Start builder
  echo "========== Starting builder container =========="
  docker run --rm -d -it \
    --name sig-builder \
    --mount type=bind,src=$ROOT,dst=/home/sigmaos/ \
    sig-builder
  buildercid=$(docker ps -a | grep -w "sig-builder" | cut -d " " -f1)
  until [ "`docker inspect -f {{.State.Running}} $buildercid`"=="true" ]; do
      echo -n "." 1>&2
      sleep 0.1;
  done
  echo "========== Done starting builder ========== "
fi

if [ -z "$rsbuildercid" ]; then
  # Build builder
  echo "========== Build Rust builder image =========="
  DOCKER_BUILDKIT=1 docker build --progress=plain -f rs-builder.Dockerfile -t sig-rs-builder . 2>&1 | tee $BUILD_LOG/sig-rs-builder.out
  echo "========== Done building Rust builder =========="
  # Start builder
  echo "========== Starting Rust builder container =========="
  docker run --rm -d -it \
    --name sig-rs-builder \
    --mount type=bind,src=$ROOT,dst=/home/sigmaos/ \
    sig-rs-builder
  rsbuildercid=$(docker ps -a | grep -w "sig-rs-builder" | cut -d " " -f1)
  until [ "`docker inspect -f {{.State.Running}} $rsbuildercid`"=="true" ]; do
      echo -n "." 1>&2
      sleep 0.1;
  done
  echo "========== Done starting Rust builder ========== "
fi


BUILD_ARGS="--norace \
  --gopath /go-custom/bin/go \
  --target $TARGET \
  $PARALLEL"

echo "========== Building kernel bins =========="
BUILD_OUT_FILE=$BUILD_LOG/make-kernel.out
docker exec -it $buildercid \
  /usr/bin/time -f "Build time: %e sec" \
  ./make.sh $BUILD_ARGS kernel \
  2>&1 | tee $BUILD_OUT_FILE && \
  if [ ${PIPESTATUS[0]} -ne 0 ]; then
    printf "\n!!!!!!!!!! BUILD ERROR !!!!!!!!!!\nLogs in: $BUILD_OUT_FILE\n" \
      | tee -a $BUILD_OUT_FILE;
  fi;
  if [ $(grep -q "BUILD ERROR" $BUILD_OUT_FILE; echo $?) -eq 0 ]; then
    echo "!!!!!!!!!! ABORTING BUILD !!!!!!!!!!"
    exit 1
  fi
  # Copy named, which is also a user bin
  cp $KERNELBIN/named $USRBIN/named
echo "========== Done building kernel bins =========="

echo "========== Building user bins =========="
BUILD_OUT_FILE=$BUILD_LOG/make-user.out
docker exec -it $buildercid \
  /usr/bin/time -f "Build time: %e sec" \
  ./make.sh $BUILD_ARGS --userbin $USERBIN user --version $VERSION \
  2>&1 | tee $BUILD_OUT_FILE && \
  if [ ${PIPESTATUS[0]} -ne 0 ]; then
    printf "\n!!!!!!!!!! BUILD ERROR !!!!!!!!!!\nLogs in: $BUILD_OUT_FILE\n" \
      | tee -a $BUILD_OUT_FILE;
  fi;
  if [ $(grep -q "BUILD ERROR" $BUILD_OUT_FILE; echo $?) -eq 0 ]; then
    echo "!!!!!!!!!! ABORTING BUILD !!!!!!!!!!"
    exit 1
  fi
echo "========== Done building user bins =========="

RS_BUILD_ARGS="--rustpath \$HOME/.cargo/bin/cargo \
  $PARALLEL"

echo "========== Building Rust bins =========="
BUILD_OUT_FILE=$BUILD_LOG/make-user-rs.out
docker exec -it $rsbuildercid \
  /usr/bin/time -f "Build time: %e sec" \
  ./make-rs.sh $RS_BUILD_ARGS --version $VERSION \
  2>&1 | tee $BUILD_OUT_FILE && \
  if [ ${PIPESTATUS[0]} -ne 0 ]; then
    printf "\n!!!!!!!!!! BUILD ERROR !!!!!!!!!!\nLogs in: $BUILD_OUT_FILE\n" \
      | tee -a $BUILD_OUT_FILE;
  fi;
  if [ $(grep -q "BUILD ERROR" $BUILD_OUT_FILE; echo $?) -eq 0 ]; then
    echo "!!!!!!!!!! ABORTING BUILD !!!!!!!!!!"
    exit 1
  fi
echo "========== Done building Rust bins =========="

echo "========== Copying kernel bins for procd =========="
if [ "${TARGET}" == "local" ]; then
  sudo cp $ROOT/create-net.sh $KERNELBIN/
  cp $KERNELBIN/procd $PROCD_BIN/
  cp $KERNELBIN/spproxyd $PROCD_BIN/
  cp $KERNELBIN/exec-uproc-rs $PROCD_BIN/
fi
echo "========== Done copying kernel bins for uproc =========="

# Now, prepare to build final containers which will actually run.
targets="sigmauser-remote sigmaos-remote"
if [ "${TARGET}" == "local" ]; then
  targets="sigmauser-local sigmaos-local"
fi

njobs=1
if ! [ -z "$PARALLEL" ]; then
  # Optionally build the docker images in parallel.
  njobs=$(echo $targets | wc -w)
fi

build_targets="parallel -j$njobs \"DOCKER_BUILDKIT=1 docker build --progress=plain -f target.Dockerfile --target {} -t {} . 2>&1 | tee $BUILD_LOG/{}.out\" ::: $targets"

printf "\nBuilding Docker image targets\n$build_targets\n\n"
echo "========== Start Docker targets build =========="
eval $build_targets
echo "========== Done building Docker targets =========="

if [ "${TARGET}" == "local" ]; then
  # If developing locally, rename the sigmaos image which includes binaries to
  # be the default sigmaos image.
  docker tag sigmaos-local sigmaos
  docker tag sigmauser-local sigmauser
else
  docker tag sigmaos-remote sigmaos
  docker tag sigmauser-remote sigmauser
  # Upload the user bins to S3
  echo "========== Pushing user bins to S3 =========="
  ./upload.sh --tag $TAG --profile sigmaos
  echo "========== Done pushing user bins to S3 =========="
fi

# Build npproxy for host
echo "========== Building proxy =========="
/usr/bin/time -f "Build time: %e sec" ./make.sh --norace $PARALLEL npproxy 
echo "========== Done building proxy =========="

if ! [ -z "$TAG" ]; then
  echo "========== Pushing container images to DockerHub =========="
  docker tag sigmaos arielszekely/sigmaos:$TAG
  docker push arielszekely/sigmaos:$TAG
  docker tag sigmauser arielszekely/sigmauser:$TAG
  docker push arielszekely/sigmauser:$TAG
fi
