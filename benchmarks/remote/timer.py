#!/usr/bin/env python3
"""
Run TestGEOCkpt in remote_test.go for N trials, then parse the benchmark output file each time
to compute the time difference between:
  - the line containing "Spawn from checkpoint nprocs"
  - the LAST occurrence of the line containing "TEST Started"

"""
import os
import argparse
import re
import subprocess
import shutil
import sys
from datetime import datetime, timedelta
from pathlib import Path
from statistics import mean

def average_single_page_times(log_dir):
    line_re = re.compile(r"copy time taken\s+(\d+).*?pages:\s+(\d+)")
    total_time = 0
    count = 0

    for entry in os.listdir(log_dir):
        path = os.path.join(log_dir, entry)
        if not os.path.isfile(path):
            continue

        with open(path, "r") as f:
            for line in f:
                match = line_re.search(line)  # use search(), not match()
                if match:
                    time_us = int(match.group(1))
                    pages = int(match.group(2))
                    if pages <4:
                        total_time += time_us
                        count += pages

    if count == 0:
        return None, 0
    return total_time / count, count
TS_RE = re.compile(r"^(?P<ts>\d{2}:\d{2}:\d{2}(?:\.\d{1,6})?)")

def parse_ts(s: str) -> datetime:
    if "." in s:
        hms, frac = s.split(".", 1)
        frac = (frac + "000000")[:6]
        s = f"{hms}.{frac}"
    else:
        s = s + ".000000"
    return datetime.strptime(s, "%H:%M:%S.%f")

def extract_ts(line: str) -> datetime | None:
    m = TS_RE.match(line)
    return parse_ts(m.group("ts")) if m else None

def compute_delta(path: Path) -> float | None:
    last_test_started, spawn = None, None
    with path.open("r", encoding="utf-8", errors="replace") as f:
        for line in f:
            if "TEST Started" in line:
                ts = extract_ts(line)
                if ts: last_test_started = ts
            if "Spawn from checkpoint nprocs" in line:
                ts = extract_ts(line)
                if ts: spawn = ts
    if not last_test_started or not spawn:
        return None
    delta = last_test_started - spawn
    return delta.total_seconds()
def non_ckpt_delta(path: Path) -> float | None:
    first_spawn_from_ckpt, shutdown = None, None
    with path.open("r", encoding="utf-8", errors="replace") as f:
        for line in f:
            if first_spawn_from_ckpt is None and "Spawn procs again" in line:
                ts = extract_ts(line)
                if ts: first_spawn_from_ckpt = ts
            if "TEST Shutdown" in line:
                ts = extract_ts(line)
                if ts: shutdown = ts
    if not first_spawn_from_ckpt or not shutdown:
        return None
    delta = shutdown- first_spawn_from_ckpt
    return delta.total_seconds()
def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--skip-ckpt", action="store_true",
                        help="skip checkpointing")
    parser.add_argument("--nprocs", type=int, default=1,
                        help="Number of procs (default: 1)")
    parser.add_argument("--nidx", type=int, default=1000,
                        help="Number of idxs (default: 1000)") 
    parser.add_argument("--trials", type=int, default=1,
                        help="Number of trials (default: 1)")
    parser.add_argument("--no-run", action="store_true",
                        help="Skip running `go test` (just parse file)")
    args = parser.parse_args()

    results = []

    for t in range(args.trials):
        trial = "1.0"+str(t)
        dir = "../results/"+trial
        dirpath = Path(dir)
        logdir = dir + "/GeoCKPT/sigmaos-node-logs"
        
        benchfile =dir+"/GeoCKPT/bench.out.0"
        path = Path(benchfile)

        if not args.no_run:
            if dirpath.exists():
                print(f"  Removing {dir} ...")
                shutil.rmtree(dirpath, ignore_errors=True)
            print(f"\n[Trial {t+1}] Running `go test`...")
            proc = subprocess.run(["go", "test","-timeout=15m","-v","-run","TestGeoCKPT","--platform","cloudlab",
                                   "--tag","freddietang","--branch","criu","-vpc","na","--version",trial,f"-nprocs={args.nprocs}",f"-skip-ckpt={args.skip_ckpt}"
                                   ,f"-n_geo_idx={args.nidx}"], text=True)
            if proc.returncode != 0:
                print(f"Warning: go test failed (exit {proc.returncode})")
        
        if not path.exists():
            print(f"Error: file not found: {path}", file=sys.stderr)
            return 2
        print("time taken per page",average_single_page_times(logdir))
        if args.skip_ckpt:
            delta= non_ckpt_delta(path)
        else:
            delta = compute_delta(path)
        if delta is None:
            print(f"[Trial {t+1}] Could not find required lines.", file=sys.stderr)
        else:
            ms = delta * 1000
            print(f"[Trial {t+1}] Delta = {ms:.3f} ms")
            results.append(ms)

    if results:
        print("\n=== Summary ===")
        print(f"Trials run: {len(results)}")
        print(f"Mean: {mean(results):.3f} ms")
        print(f"Min:  {min(results):.3f} ms")
        print(f"Max:  {max(results):.3f} ms")

    return 0

if __name__ == "__main__":
    raise SystemExit(main())