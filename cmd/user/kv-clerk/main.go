package main

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/kv"
	"ulambda/perf"
	"ulambda/proc"
)

var done = int32(0)

func main() {
	if len(os.Args) < 1 {
		db.DFatalf("Usage: %v [duration]", os.Args[0])
	}
	// Have this clerk do puts & gets instead of appends.
	var timed bool
	var dur time.Duration
	if len(os.Args) > 1 {
		timed = true
		var err error
		dur, err = time.ParseDuration(os.Args[1])
		if err != nil {
			db.DFatalf("Bad nput %v", err)
		}
	}
	clk, err := kv.MakeClerk("clerk-"+proc.GetPid().String(), fslib.Named())
	if err != nil {
		db.DFatalf("%v err %v", os.Args[0], err)
	}

	// Record performance.
	p := perf.MakePerf("KVCLERK")
	defer p.Done()

	clk.Started()
	db.DPrintf(db.ALWAYS, "KV clerk Started")
	run(clk, p, timed, dur)
}

func waitEvict(kc *kv.KvClerk) {
	err := kc.WaitEvict(proc.GetPid())
	if err != nil {
		db.DPrintf("KVCLERK", "Error WaitEvict: %v", err)
	}
	db.DPrintf("KVCLERK", "Evict\n")
	atomic.StoreInt32(&done, 1)
}

func run(kc *kv.KvClerk, p *perf.Perf, timed bool, dur time.Duration) {
	ntest := uint64(0)
	nops := uint64(0)
	var err error
	if timed {
		// Run for duration dur, then mark as done.
		go func() {
			time.Sleep(dur)
			atomic.StoreInt32(&done, 1)
		}()
	} else {
		go waitEvict(kc)
	}
	start := time.Now()
	for atomic.LoadInt32(&done) == 0 {
		// this does NKEYS puts & gets, or appends & checks, depending on whether
		// this is a time-bound clerk or an unbounded clerk.
		err = test(kc, ntest, &nops, p, timed)
		if err != nil {
			break
		}
		ntest += 1
	}
	db.DPrintf(db.ALWAYS, "%v: done ntest %v err %v\n", proc.GetName(), ntest, err)
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
	kc.Exited(status)
	db.DPrintf(db.ALWAYS, "KV clerk Exited")
}

type Value struct {
	Pid proc.Tpid
	Key kv.Tkey
	N   uint64
}

func check(kc *kv.KvClerk, key kv.Tkey, ntest uint64, p *perf.Perf) error {
	n := uint64(0)
	rdr, err := kc.GetReader(key)
	if err != nil {
		return err
	}
	rdr.Unfence()
	defer rdr.Close()
	err = fslib.JsonReader(rdr, func() interface{} { return new(Value) }, func(a interface{}) error {
		// Record op for throughput calculation.
		p.TptTick(1.0)
		val := a.(*Value)
		if val.Pid != proc.GetPid() {
			return nil
		}
		if val.Key != key {
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

func test(kc *kv.KvClerk, ntest uint64, nops *uint64, p *perf.Perf, setget bool) error {
	for i := uint64(0); i < kv.NKEYS && atomic.LoadInt32(&done) == 0; i++ {
		key := kv.MkKey(i)
		v := Value{proc.GetPid(), key, ntest}
		if setget {
			// If doing sets & gets (bounded clerk)
			if err := kc.Set(key, []byte(proc.GetPid().String()), 0); err != nil {
				return fmt.Errorf("%v: Put %v err %v", proc.GetName(), key, err)
			}
			// Record op for throughput calculation.
			p.TptTick(1.0)
			*nops++
			if _, err := kc.Get(key, 0); err != nil {
				return fmt.Errorf("%v: Get %v err %v", proc.GetName(), key, err)
			}
			// Record op for throughput calculation.
			p.TptTick(1.0)
			*nops++
		} else {
			// If doing appends (unbounded clerk)
			if err := kc.AppendJson(key, v); err != nil {
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
