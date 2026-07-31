#!/bin/bash

usage() {
  echo "Usage: $0 [--bucket BUCKET] [--profile PROFILE] DATASET..." 1>&2
  echo "" 1>&2
  echo "Stage read-only job input on this host, once, so that jobs can read it" 1>&2
  echo "straight from their local UX server. start-kernel.sh bind-mounts the" 1>&2
  echo "staging directory into the kernel container, where UX serves it, so a" 1>&2
  echo "job reads name/ux/~local/input-data/DATASET/ with no copying through" 1>&2
  echo "the namespace." 1>&2
  echo "" 1>&2
  echo "Each DATASET is an S3 prefix under BUCKET (e.g. wiki-2G), synced to" 1>&2
  echo "/tmp/sigmaos-input-data/DATASET. Already-downloaded files are skipped," 1>&2
  echo "so re-running is cheap." 1>&2
}

BUCKET="9ps3"
PROFILE="sigmaos"
DATASETS=()
while [[ $# -gt 0 ]]; do
  case "$1" in
  --bucket)
    shift
    BUCKET=$1
    shift
    ;;
  --profile)
    shift
    PROFILE=$1
    shift
    ;;
  --help)
    usage
    exit 0
    ;;
  -*)
    echo "unexpected argument $1" 1>&2
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

# Must match sp.INPUT_DATA_HOST_DIR, and the mount in start-kernel.sh.
INPUT_DATA_DIR="/tmp/sigmaos-input-data"

mkdir -p "$INPUT_DATA_DIR"
# UX runs as a different user inside the container than the one staging the
# data here, and only ever reads it.
chmod -R a+rX "$INPUT_DATA_DIR"

for dataset in "${DATASETS[@]}"; do
  # Tolerate a trailing slash, so that a dataset name can be pasted from a job
  # description's input path.
  dataset="${dataset%/}"
  dst="$INPUT_DATA_DIR/$dataset"
  echo "Staging s3://$BUCKET/$dataset -> $dst"
  mkdir -p "$dst"
  # sync rather than cp: it skips files already present with the same size and
  # mtime, so a second run over a staged dataset costs one LIST.
  if ! aws s3 sync --profile "$PROFILE" --no-progress "s3://$BUCKET/$dataset/" "$dst/"; then
    echo "!!!!!!!!!! FAILED to stage s3://$BUCKET/$dataset !!!!!!!!!!" 1>&2
    exit 1
  fi
  chmod -R a+rX "$dst"
  echo "Staged $dataset: $(du -sh "$dst" | cut -f1) in $(find "$dst" -type f | wc -l) files"
done
