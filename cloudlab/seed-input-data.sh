#!/bin/bash

usage() {
  echo "Usage: $0 [--n N] [--serial] [--bucket BUCKET] [--branch BRANCH] DATASET..." 1>&2
  echo "" 1>&2
  echo "Seed each machine's host staging directory with the named datasets, so" 1>&2
  echo "that jobs can read them straight from their local UX server rather than" 1>&2
  echo "copying them out of S3 through the namespace (see" 1>&2
  echo "../download-input-data.sh). Each DATASET is an S3 prefix under BUCKET," 1>&2
  echo "e.g. wiki-2G." 1>&2
  echo "" 1>&2
  echo "Machines are seeded in parallel by default, and each machine's download" 1>&2
  echo "is incremental, so re-running only fetches what is missing. This is the" 1>&2
  echo "one-time seed; start-sigmaos.sh --input-data keeps the data up to date on" 1>&2
  echo "every subsequent cluster start." 1>&2
  echo "" 1>&2
  echo "  --n N       seed only the first N machines" 1>&2
  echo "  --serial    seed one machine at a time (default: all in parallel)" 1>&2
  echo "  --bucket    S3 bucket holding the datasets (default: 9ps3)" 1>&2
  echo "  --branch    check this branch out on each machine before seeding" 1>&2
  echo "" 1>&2
  echo "Each machine's repo is pulled first, so that it has the downloader this" 1>&2
  echo "script runs (and any change to it); the pull output is left in /tmp/git.out" 1>&2
  echo "on the machine." 1>&2
}

N_VM=""
PARALLEL="true"
BUCKET="9ps3"
BRANCH=""
DATASETS=()
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --vpc)
    # Accepted and ignored, for symmetry with the AWS script.
    shift
    shift
    ;;
  --n)
    shift
    N_VM=$1
    shift
    ;;
  --serial)
    shift
    PARALLEL=""
    ;;
  --parallel)
    # Accepted for symmetry with the other scripts; parallel is the default.
    shift
    ;;
  --bucket)
    shift
    BUCKET=$1
    shift
    ;;
  --branch)
    shift
    BRANCH=$1
    shift
    ;;
  -help | --help)
    usage
    exit 0
    ;;
  -*)
    echo "Error: unexpected argument '$1'"
    usage
    exit 1
    ;;
  *)
    DATASETS+=("$1")
    shift
    ;;
  esac
done

if [ ${#DATASETS[@]} -eq 0 ]; then
  usage
  exit 1
fi

# Resolve servers.txt, the key, and env.sh relative to this script, so it can
# be run from anywhere rather than only from this directory.
DIR=$(cd "$(dirname "$0")" && pwd)
source $DIR/env.sh

if ! [ -f "$DIR/servers.txt" ]; then
  echo "Error: no $DIR/servers.txt — nothing to seed" 1>&2
  exit 1
fi
# servers.txt lines are "<node-name> <dns-name>", so field 2 is the DNS name we
# ssh to and field 1 is the node's name.
vms=$(cat $DIR/servers.txt | cut -d " " -f2)

vma=($vms)
if ! [ -z "$N_VM" ]; then
  vms=${vma[@]:0:$N_VM}
fi

nvm=0
for vm in $vms; do
  nvm=$(($nvm+1))
done
if [ $nvm -eq 0 ]; then
  echo "Error: no machines listed in $DIR/servers.txt" 1>&2
  exit 1
fi

# Checked out on each machine between the two pulls, so that --branch brings in
# a downloader that only exists on that branch.
CHECKOUT=""
if ! [ -z "$BRANCH" ]; then
  CHECKOUT="git checkout $BRANCH;"
fi
echo "Seeding ${#DATASETS[@]} dataset(s) [${DATASETS[@]}] from s3://$BUCKET onto $nvm machine(s)"

# Exit status per machine, so that a failure on one host is reported rather
# than lost among the parallel output.
STATUS_DIR=$(mktemp -d)
trap "rm -rf $STATUS_DIR" EXIT

i=0
# Node name and DNS name per status-file index, so a failure can identify the
# machine both ways.
VM_NAMES=()
VM_DNS=()
for vm in $vms; do
  i=$(($i+1))
  VM_NAMES[$i]=$(grep -w "$vm" $DIR/servers.txt | cut -d " " -f1)
  VM_DNS[$i]=$vm
  echo "SEED: ${VM_NAMES[$i]} ($vm)"
  seed="
    ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm <<ENDSSH
      ssh-agent bash -c 'ssh-add ~/.ssh/aws-sigmaos; (cd sigmaos; git pull > /tmp/git.out 2>&1 ; $CHECKOUT git pull >> /tmp/git.out 2>&1 )'
      cd sigmaos
      ./download-input-data.sh --bucket $BUCKET ${DATASETS[@]}
ENDSSH
    echo \$? > $STATUS_DIR/$i"
  if [ -z "$PARALLEL" ]; then
    eval "$seed"
  else
    (
      eval "$seed"
    ) &
  fi
done
wait

# One status file per machine. A missing file means that machine's ssh never
# reported, which counts as a failure rather than being skipped over.
FAILED=()
for j in $(seq 1 $nvm); do
  status="no status reported"
  if [ -f "$STATUS_DIR/$j" ]; then
    status="exit $(cat $STATUS_DIR/$j)"
    if [ "$(cat $STATUS_DIR/$j)" = "0" ]; then
      continue
    fi
  fi
  FAILED+=("${VM_NAMES[$j]} dns ${VM_DNS[$j]} ($status)")
done
if [ ${#FAILED[@]} -ne 0 ]; then
  echo "!!!!!!!!!! FAILED to seed ${#FAILED[@]}/$nvm machine(s) !!!!!!!!!!" 1>&2
  for f in "${FAILED[@]}"; do
    echo "  FAILED: $f" 1>&2
  done
  exit 1
fi
echo "Seeded $nvm machine(s)"
