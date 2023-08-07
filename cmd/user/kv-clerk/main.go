package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	cproto "sigmaos/cache/proto"

	"github.com/go-redis/redis/v8"

	"sigmaos/cache"
	db "sigmaos/debug"
	"sigmaos/kv"
	"sigmaos/kv/proto"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/semclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

var done = int32(0)
var ctx = context.Background()

func main() {
	if len(os.Args) < 2 {
		db.DFatalf("Usage: %v [duration] [keyOffset] [sempath] [redisaddr]", os.Args[0])
	}
	// Have this clerk do puts & gets instead of appends.
	var timed bool
	var dur time.Duration
	var keyOffset int
	var sempath string
	if len(os.Args) > 2 {
		timed = true
		var err error
		dur, err = time.ParseDuration(os.Args[2])
		if err != nil {
			db.DFatalf("Bad dur %v", err)
		}
		keyOffset, err = strconv.Atoi(os.Args[3])
		if err != nil {
			db.DFatalf("Bad offset %v", err)
		}
		sempath = os.Args[4]
	}
	sc, err := sigmaclnt.MkSigmaClnt(sp.Tuname("clerk-" + proc.GetPid().String()))
	if err != nil {
		db.DFatalf("MkSigmaClnt err %v", err)
	}
	var rcli *redis.Client
	var clk *kv.KvClerk
	if len(os.Args) > 5 {
		rcli = redis.NewClient(&redis.Options{
			Addr:     os.Args[5],
			Password: "",
			DB:       0,
		})
	} else {
		var err error
		clk, err = kv.MakeClerkFsl(sc.FsLib, os.Args[1])
		if err != nil {
			db.DFatalf("%v err %v", os.Args[0], err)
		}
	}

	// Record performance.
	p, err := perf.MakePerf(perf.KVCLERK)
	if err != nil {
		db.DFatalf("MakePerf err %v\n", err)
	}
	defer p.Done()

	sc.Started()
	run(sc, clk, rcli, p, timed, dur, uint64(keyOffset), sempath)
}

func waitEvict(sc *sigmaclnt.SigmaClnt, kc *kv.KvClerk) {
	err := sc.WaitEvict(proc.GetPid())
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
		sclnt := semclnt.MakeSemClnt(sc.FsLib, sempath)
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
	db.DPrintf(db.ALWAYS, "%v: done ntest %v elapsed %v err %v\n", proc.GetName(), ntest, time.Since(start), err)
	var status *proc.Status
	if err != nil {
		status = proc.MakeStatusErr(err.Error(), nil)
	} else {
		if timed {
			// If this was a bounded clerk, we should return status ok.
			d := time.Since(start)
			status = proc.MakeStatusInfo(proc.StatusOK, "ops/sec", float64(nops)/d.Seconds())
		} else {
			// If this was an unbounded clerk, we should return status evicted.
			status = proc.MakeStatus(proc.StatusEvicted)
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
		if val.Pid != proc.GetPid().String() {
			return nil
		}
		if val.Key != string(key) {
			return fmt.Errorf("%v: wrong key expected %v observed %v", proc.GetName(), key, val.Key)
		}
		if val.N != n {
			return fmt.Errorf("%v: wrong N expected %v observed %v", proc.GetName(), n, val.N)
		}
		n += 1
		return nil
	}
	if n < ntest {
		return fmt.Errorf("%v: wrong ntest expected %v observed %v", proc.GetName(), ntest, n)
	}
	return nil
}

// test performs NKEYS puts & gets, or appends & checks, depending on
// whether this is a time-bound clerk or an unbounded clerk (as
// indicated by setget).
func test(kc *kv.KvClerk, rcli *redis.Client, ntest uint64, keyOffset uint64, nops *uint64, p *perf.Perf, setget bool) error {
	for i := uint64(0); i < kv.NKEYS && atomic.LoadInt32(&done) == 0; i++ {
		key := cache.MkKey(i + keyOffset)
		v := proto.KVTestVal{Pid: proc.GetPid().String(), Key: string(key), N: ntest}
		if setget {
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
				// If doing sets & gets (bounded clerk)
				if err := kc.Put(key, &cproto.CacheString{Val: proc.GetPid().String()}); err != nil {
					return fmt.Errorf("%v: Put %v err %v", proc.GetName(), key, err)
				}
				// Record op for throughput calculation.
				p.TptTick(1.0)
				*nops++
				if err := kc.Get(key, &cproto.CacheString{}); err != nil {
					return fmt.Errorf("%v: Get %v err %v", proc.GetName(), key, err)
				}
				// Record op for throughput calculation.
				p.TptTick(1.0)
				*nops++
			}
		} else {
			// If doing appends (unbounded clerk)
			if err := kc.Append(cache.Tkey(key), &v); err != nil {
				return fmt.Errorf("%v: Append %v err %v", proc.GetName(), key, err)
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
