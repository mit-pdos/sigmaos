package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	cproto "sigmaos/apps/cache/proto"

	"github.com/go-redis/redis/v8"

	"sigmaos/apps/kv"
	"sigmaos/apps/kv/proto"
	"sigmaos/apps/cache"
	db "sigmaos/debug"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/semclnt"
	"sigmaos/sigmaclnt"
)

var done = int32(0)
var ctx = context.Background()

func main() {
	if len(os.Args) < 2 {
		db.DFatalf("Usage: %v [repl] [duration] [keyOffset] [sempath] [redisaddr]", os.Args[0])
	}
	// Have this clerk do puts & gets instead of appends.
	var timed bool
	var dur time.Duration
	var keyOffset int
	var sempath string
	if len(os.Args) > 3 {
		timed = true
		var err error
		dur, err = time.ParseDuration(os.Args[3])
		if err != nil {
			db.DFatalf("Bad dur %v", err)
		}
		keyOffset, err = strconv.Atoi(os.Args[4])
		if err != nil {
			db.DFatalf("Bad offset %v", err)
		}
		sempath = os.Args[5]
	}
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("NewSigmaClnt err %v", err)
	}
	var rcli *redis.Client
	var clk *kv.KvClerk
	if len(os.Args) > 6 {
		rcli = redis.NewClient(&redis.Options{
			Addr:     os.Args[6],
			Password: "",
			DB:       0,
		})
	} else {
		var err error
		var repl bool
		if os.Args[2] != "" {
			repl = true
		}
		clk, err = kv.NewClerkStart(sc.FsLib, os.Args[1], repl)
		if err != nil {
			db.DFatalf("%v err %v", os.Args[0], err)
		}
	}

	// Record performance.
	p, err := perf.NewPerf(sc.ProcEnv(), perf.KVCLERK)
	if err != nil {
		db.DFatalf("NewPerf err %v\n", err)
	}
	defer p.Done()

	sc.Started()
	run(sc, clk, rcli, p, timed, dur, uint64(keyOffset), sempath)
}

func waitEvict(sc *sigmaclnt.SigmaClnt, kc *kv.KvClerk) {
	err := sc.WaitEvict(sc.ProcEnv().GetPID())
	if err != nil {
		db.DPrintf(db.KVCLERK, "Error WaitEvict: %v", err)
	}
	db.DPrintf(db.KVCLERK, "Evict\n")
	atomic.StoreInt32(&done, 1)
}

func run(sc *sigmaclnt.SigmaClnt, kc *kv.KvClerk, rcli *redis.Client, p *perf.Perf, timed bool, dur time.Duration, keyOffset uint64, sempath string) {
	ntest := uint64(0)
	nops := uint64(0)
	var err error
	if timed {
		sclnt := semclnt.NewSemClnt(sc.FsLib, sempath)
		sclnt.Down()
		// Run for duration dur, then mark as done.
		go func() {
			time.Sleep(dur)
			atomic.StoreInt32(&done, 1)
		}()
	} else {
		go waitEvict(sc, kc)
	}
	start := time.Now()
	for atomic.LoadInt32(&done) == 0 {
		err = test(kc, rcli, ntest, keyOffset, &nops, p, timed)
		if err != nil {
			break
		}
		ntest += 1
	}
	d := time.Since(start)
	db.DPrintf(db.ALWAYS, "done ntest %v elapsed %v err %v\n", ntest, d, err)
	var status *proc.Status
	if err != nil {
		status = proc.NewStatusErr(err.Error(), nil)
	} else {
		if timed {
			// If this was a bounded clerk, we should return status ok.
			status = proc.NewStatusInfo(proc.StatusOK, "ops/sec", float64(nops)/d.Seconds())
		} else {
			// If this was an unbounded clerk, we should return status evicted.
			status = proc.NewStatusInfo(proc.StatusEvicted, fmt.Sprintf("ntest %d elapsed %v", ntest, d), nil)
		}
	}
	sc.ClntExit(status)
}

func check(kc *kv.KvClerk, key cache.Tkey, ntest uint64, p *perf.Perf) error {
	n := uint64(0)
	vals, err := kc.GetVals(key, &proto.KVTestVal{})
	if err != nil {
		db.DPrintf(db.ALWAYS, "GetVals err %v\n", err)
		return err
	}
	for _, v := range vals {
		val := v.(*proto.KVTestVal)
		p.TptTick(1.0)
		if val.Pid != kc.ProcEnv().GetPID().String() {
			return nil
		}
		if val.Key != string(key) {
			return fmt.Errorf("%v: wrong key expected %v observed %v", kc.ProcEnv().GetPID(), key, val.Key)
		}
		if val.N != n {
			return fmt.Errorf("%v: wrong N expected %v observed %v", kc.ProcEnv().GetPID(), n, val.N)
		}
		n += 1
		return nil
	}
	if n < ntest {
		return fmt.Errorf("%v: wrong ntest expected %v observed %v", kc.ProcEnv().GetPID(), ntest, n)
	}
	return nil
}

// test performs NKEYS puts & gets, or appends & checks, depending on
// whether this is a time-bound clerk or an unbounded clerk (as
// indicated by setget).
func test(kc *kv.KvClerk, rcli *redis.Client, ntest uint64, keyOffset uint64, nops *uint64, p *perf.Perf, setget bool) error {
	for i := uint64(0); i < kv.NKEYS && atomic.LoadInt32(&done) == 0; i++ {
		key := cache.NewKey(i + keyOffset)
		v := proto.KVTestVal{Pid: kc.ProcEnv().GetPID().String(), Key: string(key), N: ntest}
		if setget {
			// If running against redis.
			if rcli != nil {
				if err := rcli.Set(ctx, key, kc.ProcEnv().GetPID().String(), 0).Err(); err != nil {
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
				// If doing sets & gets (bounded clerk)
				if err := kc.Put(key, &cproto.CacheString{Val: kc.ProcEnv().GetPID().String()}); err != nil {
					return fmt.Errorf("%v: Put %v err %v", kc.ProcEnv().GetPID(), key, err)
				}
				// Record op for throughput calculation.
				p.TptTick(1.0)
				*nops++
				if err := kc.Get(key, &cproto.CacheString{}); err != nil {
					return fmt.Errorf("%v: Get %v err %v", kc.ProcEnv().GetPID(), key, err)
				}
				// Record op for throughput calculation.
				p.TptTick(1.0)
				*nops++
			}
		} else {
			// If doing appends (unbounded clerk)
			if err := kc.Append(cache.Tkey(key), &v); err != nil {
				return fmt.Errorf("%v: Append %v err %v", kc.ProcEnv().GetPID(), key, err)
			}
			// Record op for throughput calculation.
			p.TptTick(1.0)
			*nops++
			if err := check(kc, cache.Tkey(key), ntest, p); err != nil {
				db.DPrintf(db.ALWAYS, "check failed %v\n", err)
				return err
			}
		}
	}
	return nil
}
