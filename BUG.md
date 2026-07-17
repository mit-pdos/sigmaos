# Bug: msched/lcsched deadlock after a large MR job completes

## Summary

Running two large (29-proc) MapReduce jobs back-to-back within the same
realm/test process reliably wedges the scheduler: the second job's
coordinator proc is admitted (`Spawn`/`WaitStart` succeed on the client
side), but it never actually starts running -- no log output at all, not
even the generic proc-startup line every other proc immediately produces.
This reproduces immediately after a *fresh* `./stop.sh` restart, ruling out
cumulative resource exhaustion from a long session; it is specific to
running a second large MR job right after a first one completes.

**This is not caused by the speculative-execution work in
`apps/mr/coord.go`** -- it reproduced with the second job's `specEnabled`
both `true` and `false`, and in every case the hang happens *before* the
second coordinator ever starts, i.e. before any of that code runs at all.

## Reproduction

```go
// benchmarks/mr_straggler_bench_test.go (as it existed at time of writing)
func TestMRSpeculativeExecution(t *testing.T) {
	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{REALM1})
	...
	baseDur, baseMrst := runMRStragglerJob(mrts, StragglerSlowdownMs, false) // job 1: completes fine
	specDur, specMrst := runMRStragglerJob(mrts, StragglerSlowdownMs, true)  // job 2: coordinator never starts
	...
}
```

Command:
```
./stop.sh --parallel --nopurge --skipdb
go test -v sigmaos/benchmarks --start --run TestMRSpeculativeExecution
```

Observed every time (3+ independent runs, including immediately after a
full fresh restart):
- Job 1 (16 mappers, 13 reducers, `mr-wc-wiki1.8G.yml`) completes normally,
  logs `job done stat {...}` and `E2e bench took ...`.
- Job 2 is started by the client (`StartMRJob` -> `Spawn`/`WaitStart`
  succeed), then the client blocks forever in `ji.Wait()` ->
  `mr.WaitJobDone` -> semaphore `Down()`.
- `./logs.sh` shows only **one** `exec: /mnt/binfs/mr-coord-v1.0 [...]`
  line for the entire run -- job 2's coordinator is never exec'd.
- The test hangs until Go's `-test.timeout` (10m) fires, or until manually
  interrupted.

## Root cause (from goroutine dumps, SIGQUIT on both the test client and lcsched)

Client-side stack (stuck in `ji.StartMRJob()`'s underlying spawn call for
job 2's coordinator):
```
sigmaos/sigmaclnt/procclnt.(*ProcClnt).spawnRetry
  -> enqueueViaLCSched
  -> sched/lcsched/clnt.(*LCSchedClnt).Enqueue   (RPC)
  -> blocked on channel receive, waiting for a response that never arrives
```

lcsched-side stack (obtained by `docker exec <node> kill -QUIT <lcsched-pid>`):
```
sched/lcsched/srv.(*LCSched).Enqueue
  -> srv.go:65, blocked on an internal channel receive (waiting for a free
     admission slot)

sched/lcsched/srv.(*LCSched).schedule
  -> runProc -> waitProcExit (srv.go:171/177)
  -> sched/msched/clnt.(*MSchedClnt).Wait(pid)   (RPC to msched)
  -> blocked on channel receive, waiting for msched's response
```

So the chain is:

1. lcsched's own admission-control loop is waiting on `waitProcExit` for
   *some* proc from job 1 to be reported as exited by msched, so it can
   free up a slot.
2. That `msched.Wait(pid)` RPC never returns -- msched is alive (confirmed
   via `docker exec ... ps aux`, not crashed, using ~0% CPU) but never
   answers this specific wait request.
3. Because that slot never frees, lcsched's `Enqueue` for job 2's
   coordinator blocks forever on its own internal channel.
4. The client's `Spawn`/`WaitStart` for job 2's coordinator therefore also
   blocks forever (retries via `spawnRetry` never succeed).

Also visible in every occurrence: multiple simultaneous `Sess ... timed
out` `ALWAYS`-level log lines from **every** kernel service (knamed,
msched, realmd, lcsched, the node itself) around the same timestamp -- this
looks like a broader session-liveness hiccup that coincides with (and is
possibly the trigger for) msched failing to answer the specific
`waitProcExit` wait.

## What's NOT the cause

- Not related to `apps/mr/coord.go`'s new speculative-execution code:
  reproduced with job 2's `specEnabled=false` too, and the hang occurs
  strictly before job 2's coordinator process ever starts (so none of
  `speculate`/`speculateMap`/`speculateReduce`/`runBackupMap`/
  `runBackupReduce`/`taskWon` ever executes).
- Not a pid/job-name collision (fixed separately: job names must include a
  random suffix per job instance, or `InitCoordFS`'s `MkDir` fails with
  `file exists` on the second job -- that was a distinct, already-fixed bug
  in the test helper, unrelated to this deadlock).
- Not simple resource exhaustion from a long test session: reproduces on
  the very first test run immediately after a full `./stop.sh` restart.

## Suspected trigger

The specific pattern that triggers it is a **large number of procs
finishing in a short window** -- job 1's final ~15 tasks complete within
about 10-15 seconds of each other (per the `tasks done N/29` log
timestamps), which is exactly when the "Sess ... timed out" lines appear
across every kernel service. This smells like a burst-load issue in the
session/RPC layer (`sigmaclnt/procclnt`, `session/clnt`,
`sched/msched`) under a rapid burst of proc-exit notifications, rather than
anything specific to MapReduce.

## Workaround adopted for this branch

`TestMRSpeculativeExecution` was reverted from "run baseline + speculative
back-to-back in one test" (which needs two large sequential MR jobs in one
realm, and hits this bug) back to a standalone test with its own fresh
realm, compared against `TestMRStragglerBaseline`'s separately-printed
numbers rather than an in-process comparison. This avoids the bug
entirely rather than fixing it -- fixing `sched/msched`/`sched/lcsched`
itself is out of scope for this branch (`urop/mr-straggler-baseline`,
about `apps/mr` straggler mitigation, not core scheduler internals).

## Suggested follow-up (not done here)

- Reproduce outside `apps/mr` entirely (e.g. spawn ~29 short-lived procs
  back-to-back via a minimal test) to confirm this is a general
  msched/lcsched issue and not specific to MR's proc shape/timing.
- Add debug logging (`SIGMADEBUG` selector) inside `msched`'s wait-request
  handling and `lcsched`'s admission loop to see why a specific
  `waitProcExit` request is never answered -- e.g. whether the proc it's
  waiting on already exited and the notification was dropped, versus
  msched itself being stuck.
- Look at whatever caused the simultaneous `Sess ... timed out` messages
  across every kernel service at the same moment -- if that's the trigger,
  fixing it may fix the deadlock as a side effect.
