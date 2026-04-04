#!/bin/bash

set -eo pipefail

usage() {
  echo "Usage: $0 [--push TAG] [--target TARGET] [--version VERSION] [--userbin USERBIN] [-j NJOBS] [--no_go] [--no_go_user] [--no_rs] [--no_docker] [--no_cpp] [--no_py]  [--refresh_gvisor_bundle] [--rebuildbuilder] [--nocache] [--debug]" 1>&2
}

NJOBS="$(nproc)"
REBUILD_BUILDER="false"
NO_CACHE=""
TAG=""
TARGET="local"
VERSION="1.0"
USERBIN="all"
NO_CPP="false"
NO_RS="false"
NO_GO="false"
NO_GO_USER="false"
NO_PY="false"
NO_DOCKER="false"
REFRESH_GVISOR_BUNDLE="false"
NORACE="--norace"
DEBUG=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  -j)
    shift
    NJOBS="$1"
    shift
    ;;
  --rebuildbuilder)
    shift
    REBUILD_BUILDER="true"
    ;;
  --no_docker)
    shift
    NO_DOCKER="true"
    ;;
  --no_go)
    shift
    NO_GO="true"
    ;;
  --no_go_user)
    shift
    NO_GO_USER="true"
    ;;
  --no_rs)
    shift
    NO_RS="true"
    ;;
  --no_cpp)
    shift
    NO_CPP="true"
    ;;
  --no_py)
    shift
    NO_PY="true"
    ;;
  --nocache)
    shift
    NO_CACHE="--no-cache"
    ;;
  --refresh_gvisor_bundle)
    shift
    REFRESH_GVISOR_BUNDLE="true"
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
  --race)
    shift
    NORACE=""
    shift
    ;;
  --debug)
    shift
    DEBUG="true"
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

ROOT=$(dirname $(realpath $0))
source $ROOT/env/env.sh

TMP_BASE="/tmp"
BUILDER_NAME="sig-builder"
RS_BUILDER_NAME="sig-rs-builder"
PY_BUILDER_NAME="sig-py-builder"
CPP_BUILDER_NAME="sig-cpp-builder"
USER_IMAGE_NAME="sigmauser"
KERNEL_IMAGE_NAME="sigmaos"
BUILD_TARGET_SUFFIX=""
if ! [ -z "$SIGMAUSER" ]; then
  TMP_BASE=$TMP_BASE/$SIGMAUSER
  BUILDER_NAME=$BUILDER_NAME-$SIGMAUSER
  RS_BUILDER_NAME=$RS_BUILDER_NAME-$SIGMAUSER
  PY_BUILDER_NAME=$PY_BUILDER_NAME-$SIGMAUSER
  CPP_BUILDER_NAME=$CPP_BUILDER_NAME-$SIGMAUSER
  USER_IMAGE_NAME=$USER_IMAGE_NAME-$SIGMAUSER
  KERNEL_IMAGE_NAME=$KERNEL_IMAGE_NAME-$SIGMAUSER
  BUILD_TARGET_SUFFIX="-$SIGMAUSER"
fi

BUILD_LOG="${TMP_BASE}/sigmaos-build"
PROCD_BIN="${TMP_BASE}/sigmaos-procd-bin"
PYTHON="${TMP_BASE}/python"

# tests uses host's /tmp, which mounted in kernel container.
mkdir -p $TMP_BASE
mkdir -p $BUILD_LOG
mkdir -p $PYTHON

# Make a dir to hold user proc build output
BIN=${ROOT}/bin
KERNELBIN=${BIN}/kernel
USRBIN=${BIN}/user
mkdir -p $KERNELBIN
mkdir -p $USRBIN
if [ "${NO_GO}" != "true" ]; then
  # Clear the procd bin directory if rebuilding Go procs
  rm -rf $PROCD_BIN
fi
mkdir -p $PROCD_BIN

# Shell helper that runs a command, mirrors output to a log, and exits if it fails.
run_with_log() {
  local step="$1"
  local logfile="$2"
  shift 2
  if ! { "$@" 2>&1 | tee "$logfile"; }; then
    local exit_code=${PIPESTATUS[0]:-1}
    echo ""
    echo "!!!!!!!!!! BUILD ERROR: $step !!!!!!!!!!"
    echo "Logs in: $logfile"
    echo "!!!!!!!!!! ABORTING BUILD !!!!!!!!!!"
    exit $exit_code
  fi
}

# build and start db container
if [ "${TARGET}" != "remote" ]; then
    ./start-network.sh
fi

# Function to ensure a builder container is running; restarts or builds as needed.
# Args:
#   1: variable name to store resulting container id (e.g., buildercid)
#   2: container/image name (e.g., $BUILDER_NAME)
#   3: dockerfile path (e.g., docker/builder.Dockerfile)
#   4: log basename (e.g., sig-builder)
ensure_builder() {
  local out_var="$1"
  local name="$2"
  local dockerfile="$3"
  local logbase="$4"
  local cid

  # Find existing container ID (exact name match)
  cid=$(docker ps -aq --filter name="^/${name}$" || true)

  # If forced rebuild, stop existing container
  if [[ "$REBUILD_BUILDER" == "true" && -n "$cid" ]]; then
    echo "========== Stopping old ${name} container ${cid} =========="
    docker stop "$cid" || true
    cid=""
  fi

  # Try to restart existing container if present but not running
  if [ -n "$cid" ]; then
    local running
    running=$(docker inspect -f '{{.State.Running}}' "$cid" 2>/dev/null || echo false)
    if [ "$running" != "true" ]; then
      echo "========== ${name} exists but not running; attempting restart =========="
      if docker start "$cid" >/dev/null 2>&1; then
        until [ "$(docker inspect -f '{{.State.Running}}' "$cid")" == "true" ]; do
          echo -n "." 1>&2; sleep 0.1;
        done
        echo "========== ${name} restarted =========="
      else
        cid=""
      fi
    fi
  fi

  # If no container running, try to run from existing image (unless forced rebuild)
  if [ -z "$cid" ] && [ "$REBUILD_BUILDER" != "true" ]; then
    if docker image inspect "$name" >/dev/null 2>&1; then
      echo "========== No ${name} running; starting from existing image =========="
      if docker run --rm -d -it \
        --name "$name" \
        --mount type=bind,src="$ROOT",dst=/home/sigmaos/ \
        "$name"; then
        cid=$(docker ps -aq --filter name="^/${name}$")
        until [ "$(docker inspect -f '{{.State.Running}}' "$cid")" == "true" ]; do
          echo -n "." 1>&2; sleep 0.1;
        done
        echo "========== Started ${name} from existing image =========="
      else
        echo "Could not start ${name} from existing image; will rebuild image."
      fi
    else
      echo "${name} image not found; will build image."
    fi
  fi

  # Build image and run container if still not running
  if [ -z "$cid" ]; then
    echo "========== Build ${name} image =========="
    DOCKER_BUILDKIT=1 docker build $NO_CACHE \
      --progress=plain \
      --build-arg USER_ID=$(id -u) \
      --build-arg GROUP_ID=$(id -g) \
      -f "$dockerfile" \
      -t "$name" \
      . 2>&1 | tee "$BUILD_LOG/${logbase}.out"
    echo "========== Done building ${name} =========="
    echo "========== Starting ${name} container =========="
    docker run --rm -d -it \
      --name "$name" \
      --mount type=bind,src="$ROOT",dst=/home/sigmaos/ \
      "$name"
    cid=$(docker ps -aq --filter name="^/${name}$")
    until [ "$(docker inspect -f '{{.State.Running}}' "$cid")" == "true" ]; do
      echo -n "." 1>&2; sleep 0.1;
    done
    echo "========== Done starting ${name} ========== "
  fi

  # Export resulting container id to requested variable name
  if [ -n "$out_var" ]; then
    printf -v "$out_var" '%s' "$cid"
  fi
}

ensure_builder buildercid "$BUILDER_NAME" docker/builder.Dockerfile sig-builder
ensure_builder cppbuildercid "$CPP_BUILDER_NAME" docker/cpp-builder.Dockerfile sig-cpp-builder
ensure_builder rsbuildercid "$RS_BUILDER_NAME" docker/rs-builder.Dockerfile sig-rs-builder
ensure_builder pybuildercid "$PY_BUILDER_NAME" docker/py-builder.Dockerfile sig-py-builder

if [ "${NO_GO}" != "true" ]; then
  BUILD_ARGS="\
    $NORACE \
    --gopath /go-custom/bin/go \
    --target $TARGET \
    -j $NJOBS"

  echo "========== Building kernel bins =========="
  BUILD_OUT_FILE=$BUILD_LOG/make-kernel.out
  run_with_log "make kernel" "$BUILD_OUT_FILE" docker exec $buildercid \
    /usr/bin/time -f "Build time: %e sec" \
    ./make.sh $BUILD_ARGS kernel
  # Copy named, which is also a user bin
  cp $KERNELBIN/named $USRBIN/named
  echo "========== Done building kernel bins =========="

  if [ "${NO_GO_USER}" != "true" ]; then
    echo "========== Building user bins =========="
    BUILD_OUT_FILE=$BUILD_LOG/make-user.out
    run_with_log "make user" "$BUILD_OUT_FILE" docker exec $buildercid \
      /usr/bin/time -f "Build time: %e sec" \
      ./make.sh $BUILD_ARGS --userbin $USERBIN user --version $VERSION
    echo "========== Done building user bins =========="
  fi
fi

if [ "${NO_RS}" != "true" ]; then
  RS_BUILD_ARGS="--rustpath \$HOME/.cargo/bin/cargo \
    -j $NJOBS"

  echo "========== Building Rust bins =========="
  BUILD_OUT_FILE=$BUILD_LOG/make-user-rs.out
  run_with_log "make rust" "$BUILD_OUT_FILE" docker exec $rsbuildercid \
    /usr/bin/time -f "Build time: %e sec" \
    ./make-rs.sh $RS_BUILD_ARGS --version $VERSION
  echo "========== Done building Rust bins =========="
fi

if [ "${NO_CPP}" != "true" ]; then
  CPP_BUILD_ARGS="$( if [ "$DEBUG" == "true" ]; then echo "--build_type Debug"; fi )\
    -j $NJOBS"

  echo "========== Building CPP bins =========="
  BUILD_OUT_FILE=$BUILD_LOG/make-user-cpp.out
  run_with_log "make cpp" "$BUILD_OUT_FILE" docker exec $cppbuildercid \
    /usr/bin/time -f "Build time: %e sec" \
    ./make-cpp.sh $CPP_BUILD_ARGS --version $VERSION
  echo "========== Done building CPP bins =========="
fi

if [ "${NO_PY}" != "true" ]; then
  echo "========== Building Python bins =========="
  BUILD_OUT_FILE=$BUILD_LOG/make-user-py.out
  run_with_log "make python" "$BUILD_OUT_FILE" docker exec $pybuildercid \
    /usr/bin/time -f "Build time: %e sec" \
    ./make-python.sh --version $VERSION
  echo "========== Done building Python bins =========="
fi

if [ "${NO_DOCKER}" != "true" ]; then
  echo "========== Copying kernel bins for procd =========="
  if [ "${TARGET}" == "local" ]; then
    cp $ROOT/create-net.sh $KERNELBIN/
    cp $KERNELBIN/procd $PROCD_BIN/
    cp $KERNELBIN/spproxyd $PROCD_BIN/
    cp $KERNELBIN/uproc-trampoline $PROCD_BIN/
    cp $KERNELBIN/scontainer $PROCD_BIN/

    cp -r $KERNELBIN/cpython* $PROCD_BIN/
    cp -r $KERNELBIN/pyproc $PROCD_BIN/   # TODO: Use binfs instead of shipping pyproc with the kernel
  fi
  echo "========== Done copying kernel bins for proc =========="
fi

# Now, prepare to build final containers which will actually run.
targets="sigmauser-remote sigmaos-remote"
if [ "${TARGET}" == "local" ]; then
  targets="sigmauser-local sigmaos-local"
fi

if [ "${NO_DOCKER}" != "true" ]; then
  echo "========== Start Docker targets build =========="
  parallel --verbose -j"$NJOBS" --tag \
    "DOCKER_BUILDKIT=1 docker build --progress=plain -f docker/target.Dockerfile --target {} -t {}$BUILD_TARGET_SUFFIX . 2>&1 | tee $BUILD_LOG/{}.out" ::: $targets
  docker_build_status=$?
  if [ $docker_build_status -ne 0 ]; then
    printf "\n!!!!!!!!!! BUILD ERROR !!!!!!!!!!\n";
    echo "!!!!!!!!!! ABORTING BUILD !!!!!!!!!!"
    exit 1
  fi
  echo "========== Done building Docker targets =========="
fi

if [ "${TARGET}" == "local" ]; then
  # If developing locally, rename the sigmaos image which includes binaries to
  # be the default sigmaos image.
  docker tag sigmaos-local$BUILD_TARGET_SUFFIX $KERNEL_IMAGE_NAME
  docker tag sigmauser-local$BUILD_TARGET_SUFFIX $USER_IMAGE_NAME
else
  docker tag sigmaos-remote $KERNEL_IMAGE_NAME
  docker tag sigmauser-remote $USER_IMAGE_NAME
  # Upload the user bins to S3
  echo "========== Pushing user bins to S3 =========="
  ./upload.sh --tag $TAG --profile sigmaos
  echo "========== Done pushing user bins to S3 =========="
fi

if [ "${NO_GO}" != "true" ]; then
  # Build npproxy for host
  echo "========== Building proxy =========="
  /usr/bin/time -f "Build time: %e sec" ./make.sh --norace -j $NJOBS npproxy
  echo "========== Done building proxy =========="
fi

if ! [ -z "$TAG" ]; then
  echo "========== Pushing container images to DockerHub =========="
  docker tag sigmaos arielszekely/sigmaos:$TAG
  docker push arielszekely/sigmaos:$TAG
  docker tag sigmauser arielszekely/sigmauser:$TAG
  docker push arielszekely/sigmauser:$TAG
  echo "========== Done pushing container images to DockerHub =========="
fi

# If gVisor rootfs doesn't exist, create it
GVISOR_BUNDLE=/tmp/sigmaos-base-user-bundle
if [ "${REFRESH_GVISOR_BUNDLE}" == "true" ] || ! [ -d $GVISOR_BUNDLE ]; then
  echo "========== Create GVisor Bundle =========="
  ./create-gvisor-bundle.sh
  echo "========== Done creating GVisor Bundle =========="
fi
