#!/bin/bash

usage() {
  echo "Usage: $0 [--merge]" 1>&2
}

MERGE="nomerge"
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --merge)
    shift
    MERGE="merge"
    ;;
   *)
   echo "unexpected argument $1"
   usage
   exit 1
 esac
done

out=""
for containerid in $(docker ps -a --format "{{.Names}}"); do
  if [[ $containerid == sigma-* ]] || [[ $containerid == kernel-* ]]; then
    ctr_out="$(docker logs $containerid 2>&1)"
    if [[ "$MERGE" == "merge" ]] ; then
      out="$(printf "%s\n" "$out" "$ctr_out")"
    else
      out="$(printf "%s\n" "$out" "========== Logs for $containerid ==========" "$ctr_out")"
    fi
  fi
done

# Scrape blinkd logs
if [ -f /tmp/blinkd.out ]; then
  blinkd_out="$(cat /tmp/blinkd.out 2>&1)"
  out="$(printf "%s\n" "$out" "========== Logs for blinkd ==========" "$blinkd_out")"
fi

if [ -f /tmp/blinkd-restore.out ]; then
  blinkd_out="$(cat /tmp/blinkd-restore.out 2>&1)"
  out="$(printf "%s\n" "$out" "========== Logs for blinkd-restore ==========" "$blinkd_out")"
fi

# Trim first line (which is blank)
out="$(echo "$out" | tail -n +2 )"
if [[ "$MERGE" == "merge" ]] ; then
  out="$(echo "$out" | sort -k 1)"
fi
echo "$out"
echo "nproc: $(nproc)"
