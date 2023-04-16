package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/kv"
	"sigmaos/kv/proto"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/semclnt"
	"sigmaos/sigmaclnt"
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
	sc, err := sigmaclnt.MkSigmaClnt("clerk-" + proc.GetPid().String())
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
	run(sc.ProcClnt, clk, rcli, p, timed, dur, uint64(keyOffset), sempath)
}

func waitEvict(pclnt *procclnt.ProcClnt, kc *kv.KvClerk) {
	err := pclnt.WaitEvict(proc.GetPid())
	if err != nil {
		db.DPrintf(db.KVCLERK, "Error WaitEvict: %v", err)
	}
	db.DPrintf(db.KVCLERK, "Evict\n")
	atomic.StoreInt32(&done, 1)
}

func run(pclnt *procclnt.ProcClnt, kc *kv.KvClerk, rcli *redis.Client, p *perf.Perf, timed bool, dur time.Duration, keyOffset uint64, sempath string) {
	ntest := uint64(0)
	nops := uint64(0)
	var err error
	if timed {
		sclnt := semclnt.MakeSemClnt(pclnt.FsLib, sempath)
		sclnt.Down()
		// Run for duration dur, then mark as done.
		go func() {
			time.Sleep(dur)
			atomic.StoreInt32(&done, 1)
		}()
	} else {
		go waitEvict(pclnt, kc)
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
	pclnt.Exited(status)
}

func check(kc *kv.KvClerk, key kv.Tkey, ntest uint64, p *perf.Perf) error {
	n := uint64(0)
	rdr, err := kc.GetReader(key)
	if err != nil {
		return err
	}
	rdr.Unfence()
	defer rdr.Close()
	err = fslib.JsonReader(rdr, func() interface{} { return new(proto.KVTestVal) }, func(a interface{}) error {
		// Record op for throughput calculation.
		p.TptTick(1.0)
		val := a.(*proto.KVTestVal)
		if val.Pid != proc.GetPid().String() {
			return nil
		}
		if val.Key != string(key) {
			return fmt.Errorf("%v: wrong key for %v: expected %v observed %v", proc.GetName(), rdr.Path(), key, val.Key)
		}
		if val.N != n {
			return fmt.Errorf("%v: wrong N for %v: expected %v observed %v", proc.GetName(), rdr.Path(), n, val.N)
		}
		n += 1
		return nil
	})
	if err != nil {
		db.DPrintf(db.ALWAYS, "JsonReader: err %v\n", err)
	}
	if n < ntest {
		return fmt.Errorf("%v: wrong ntest for %v: expected %v observed %v", proc.GetName(), rdr.Path(), ntest, n)
	}
	return nil
}

// test performs NKEYS puts & gets, or appends & checks, depending on
// whether this is a time-bound clerk or an unbounded clerk (as
// indicated by setget).
func test(kc *kv.KvClerk, rcli *redis.Client, ntest uint64, keyOffset uint64, nops *uint64, p *perf.Perf, setget bool) error {
	for i := uint64(0); i < kv.NKEYS && atomic.LoadInt32(&done) == 0; i++ {
		key := kv.MkKey(i + keyOffset)
		v := proto.KVTestVal{Pid: proc.GetPid().String(), Key: string(key), N: ntest}
		if setget {
			// If running against redis.
			if rcli != nil {
				if err := rcli.Set(ctx, key.String(), proc.GetPid().String(), 0).Err(); err != nil {
					db.DFatalf("Error redis cli set: %v", err)
				}
				// Record op for throughput calculation.
				p.TptTick(1.0)
				*nops++
				if _, err := rcli.Get(ctx, key.String()).Result(); err != nil {
					db.DFatalf("Error redis cli get: %v", err)
				}
				// Record op for throughput calculation.
				p.TptTick(1.0)
				*nops++
			} else {
				// If doing sets & gets (bounded clerk)
				if err := kc.PutRaw(key, []byte(proc.GetPid().String()), 0); err != nil {
					return fmt.Errorf("%v: Put %v err %v", proc.GetName(), key, err)
				}
				// Record op for throughput calculation.
				p.TptTick(1.0)
				*nops++
				if _, err := kc.GetRaw(key, 0); err != nil {
					return fmt.Errorf("%v: Get %v err %v", proc.GetName(), key, err)
				}
				// Record op for throughput calculation.
				p.TptTick(1.0)
				*nops++
			}
		} else {
			// If doing appends (unbounded clerk)
			if err := kc.AppendJson(key, &v); err != nil {
				return fmt.Errorf("%v: Append %v err %v", proc.GetName(), key, err)
			}
			// Record op for throughput calculation.
			p.TptTick(1.0)
			*nops++
			if err := check(kc, key, ntest, p); err != nil {
				db.DPrintf(db.ALWAYS, "check failed %v\n", err)
				return err
			}
		}
	}
	return nil
}
