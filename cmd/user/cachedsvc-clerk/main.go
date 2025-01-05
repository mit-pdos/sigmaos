package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"

	cachegrpclnt "sigmaos/apps/cache/cachegrp/clnt"
	cacheclnt "sigmaos/apps/cache/clnt"
	"sigmaos/apps/cache/proto"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/util/coordination/semaphore"
	"sigmaos/sigmaclnt"
	"sigmaos/util/perf"
)

var done = int32(0)
var ctx = context.Background()

func main() {
	if len(os.Args) < 5 {
		db.DFatalf("Usage: %v job keys duration keyOffset sempath [redisaddr]", os.Args[0])
	}
	var dur time.Duration
	var nkeys int
	var keyOffset int
	var sempath string
	var err error
	nkeys, err = strconv.Atoi(os.Args[2])
	if err != nil {
		db.DFatalf("Bad nkeys %v", err)
	}
	dur, err = time.ParseDuration(os.Args[3])
	if err != nil {
		db.DFatalf("Bad dur %v", err)
	}
	keyOffset, err = strconv.Atoi(os.Args[4])
	if err != nil {
		db.DFatalf("Bad offset %v", err)
	}
	sempath = os.Args[5]
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("NewSigmaClnt err %v", err)
	}
	var rcli *redis.Client
	var csc *cachegrpclnt.CachedSvcClnt
	if len(os.Args) > 6 {
		rcli = redis.NewClient(&redis.Options{
			Addr:     os.Args[6],
			Password: "",
			DB:       0,
		})
	} else {
		csc = cachegrpclnt.NewCachedSvcClnt(sc.FsLib, os.Args[1])
	}

	// Record performance.
	p, err := perf.NewPerf(sc.ProcEnv(), perf.CACHECLERK)
	if err != nil {
		db.DFatalf("NewPerf err %v\n", err)
	}
	defer p.Done()

	sc.Started()
	run(sc, csc, rcli, p, dur, nkeys, uint64(keyOffset), sempath)
}

func run(sc *sigmaclnt.SigmaClnt, csc *cachegrpclnt.CachedSvcClnt, rcli *redis.Client, p *perf.Perf, dur time.Duration, nkeys int, keyOffset uint64, sempath string) {
	ntest := uint64(0)
	nops := uint64(0)
	var err error
	sclnt := semaphore.NewSemaphore(sc.FsLib, sempath)
	sclnt.Down()
	// Run for duration dur, then mark as done.
	go func() {
		time.Sleep(dur)
		atomic.StoreInt32(&done, 1)
	}()
	start := time.Now()
	for atomic.LoadInt32(&done) == 0 {
		// this does NKEYS puts & gets, or appends & checks, depending on whether
		// this is a time-bound clerk or an unbounded clerk.
		err = test(sc, csc, rcli, ntest, nkeys, keyOffset, &nops, p)
		if err != nil {
			break
		}
		ntest += 1
	}
	db.DPrintf(db.ALWAYS, "done ntest %v elapsed %v err %v\n", ntest, time.Since(start), err)
	var status *proc.Status
	if err != nil {
		status = proc.NewStatusErr(err.Error(), nil)
	} else {
		d := time.Since(start)
		status = proc.NewStatusInfo(proc.StatusOK, "ops/sec", float64(nops)/d.Seconds())
	}
	if csc != nil {
		csc.Close()
	}
	sc.ClntExit(status)
}

func test(sc *sigmaclnt.SigmaClnt, csc *cachegrpclnt.CachedSvcClnt, rcli *redis.Client, ntest uint64, nkeys int, keyOffset uint64, nops *uint64, p *perf.Perf) error {
	for i := uint64(0); i < uint64(nkeys) && atomic.LoadInt32(&done) == 0; i++ {
		key := cacheclnt.NewKey(i + keyOffset)
		// If running against redis.
		if rcli != nil {
			if err := rcli.Set(ctx, key, sc.ProcEnv().GetPID().String(), 0).Err(); err != nil {
				db.DFatalf("Error redis cli set: %v", err)
			}
			// Record op for throughput calculation.
			p.TptTick(1.0)
			*nops++
			if _, err := rcli.Get(ctx, key).Result(); err != nil {
				db.DFatalf("Error redis cli get: %v", err)
			}
			// Record op for throughput calculation.
			p.TptTick(1.0)
			*nops++
		} else {
			if err := csc.Put(key, &proto.CacheString{Val: sc.ProcEnv().GetPID().String()}); err != nil {
				return fmt.Errorf("%v: Put %v err %v", sc.ProcEnv().GetPID(), key, err)
			}
			// Record op for throughput calculation.
			p.TptTick(1.0)
			*nops++
			if err := csc.Get(key, &proto.CacheString{}); err != nil {
				db.DPrintf(db.ALWAYS, "miss %v", key)
				// return fmt.Errorf("%v: Get %v err %v", key, err)
			}
			// Record op for throughput calculation.
			p.TptTick(1.0)
			*nops++
		}
	}
	return nil
}
