package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"

	proto "sigmaos/cache/proto"
	"sigmaos/cacheclnt"
	"sigmaos/cachesrv"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/semclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
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
	sc, err := sigmaclnt.MkSigmaClnt(sp.Tuname("cacheclerk-" + proc.GetPid().String()))
	if err != nil {
		db.DFatalf("MkSigmaClnt err %v", err)
	}
	var rcli *redis.Client
	var cc *cacheclnt.CacheClnt
	if len(os.Args) > 6 {
		rcli = redis.NewClient(&redis.Options{
			Addr:     os.Args[6],
			Password: "",
			DB:       0,
		})
	} else {
		var err error
		cc, err = cacheclnt.MkCacheClnt([]*fslib.FsLib{sc.FsLib}, os.Args[1], cachesrv.NSHARD)
		if err != nil {
			db.DFatalf("%v err %v", os.Args[0], err)
		}
	}

	// Record performance.
	p, err := perf.MakePerf(perf.CACHECLERK)
	if err != nil {
		db.DFatalf("MakePerf err %v\n", err)
	}
	defer p.Done()

	sc.Started()
	run(sc, cc, rcli, p, dur, nkeys, uint64(keyOffset), sempath)
}

func waitEvict(cc *cacheclnt.CacheClnt, pclnt *procclnt.ProcClnt) {
	err := pclnt.WaitEvict(proc.GetPid())
	if err != nil {
		db.DPrintf(db.CACHECLERK, "Error WaitEvict: %v", err)
	}
	db.DPrintf(db.CACHECLERK, "Evict\n")
	atomic.StoreInt32(&done, 1)
}

func run(sc *sigmaclnt.SigmaClnt, cc *cacheclnt.CacheClnt, rcli *redis.Client, p *perf.Perf, dur time.Duration, nkeys int, keyOffset uint64, sempath string) {
	ntest := uint64(0)
	nops := uint64(0)
	var err error
	sclnt := semclnt.MakeSemClnt(sc.FsLib, sempath)
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
		err = test(cc, rcli, ntest, nkeys, keyOffset, &nops, p)
		if err != nil {
			break
		}
		ntest += 1
	}
	db.DPrintf(db.ALWAYS, "%v: done ntest %v elapsed %v err %v\n", proc.GetName(), ntest, time.Since(start), err)
	var status *proc.Status
	if err != nil {
		status = proc.MakeStatusErr(err.Error(), nil)
	} else {
		d := time.Since(start)
		status = proc.MakeStatusInfo(proc.StatusOK, "ops/sec", float64(nops)/d.Seconds())
	}
	sc.ClntExit(status)
}

func test(cc *cacheclnt.CacheClnt, rcli *redis.Client, ntest uint64, nkeys int, keyOffset uint64, nops *uint64, p *perf.Perf) error {
	for i := uint64(0); i < uint64(nkeys) && atomic.LoadInt32(&done) == 0; i++ {
		key := cacheclnt.MkKey(i + keyOffset)
		// If running against redis.
		if rcli != nil {
			if err := rcli.Set(ctx, key, proc.GetPid().String(), 0).Err(); err != nil {
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
			if err := cc.Put(key, &proto.CacheString{Val: proc.GetPid().String()}); err != nil {
				return fmt.Errorf("%v: Put %v err %v", proc.GetName(), key, err)
			}
			// Record op for throughput calculation.
			p.TptTick(1.0)
			*nops++
			if err := cc.Get(key, &proto.CacheString{}); err != nil {
				db.DPrintf(db.ALWAYS, "miss %v", key)
				// return fmt.Errorf("%v: Get %v err %v", proc.GetName(), key, err)
			}
			// Record op for throughput calculation.
			p.TptTick(1.0)
			*nops++
		}
	}
	return nil
}
