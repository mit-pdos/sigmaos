package main

import (
	"fmt"
	"os"
	"strconv"
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
		db.DFatalf("%v too few args", os.Args[0])
	}
	// Have this clerk do puts & gets instead of appends.
	var putget bool
	var nputget int
	if len(os.Args) > 1 {
		putget = true
		var err error
		nputget, err = strconv.Atoi(os.Args[1])
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
	run(clk, p, putget, nputget)
}

func waitEvict(kc *kv.KvClerk) {
	err := kc.WaitEvict(proc.GetPid())
	if err != nil {
		db.DPrintf("KVCLERK", "Error WaitEvict: %v", err)
	}
	db.DPrintf("KVCLERK", "Evict\n")
	atomic.StoreInt32(&done, 1)
}

func run(kc *kv.KvClerk, p *perf.Perf, putget bool, nputget int) {
	ntest := uint64(0)
	var err error
	go waitEvict(kc)
	start := time.Now()
	// Run until we've done nputget puts & gets (if this is a bounded clerk) or
	// we are done (otherwise).
	for i := 0; (putget && i/kv.NKEYS < nputget) || (!putget && atomic.LoadInt32(&done) == 0); i++ {
		// this does NKEYS puts & gets, or appends & checks.
		err = test(kc, ntest, p, putget)
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
		if putget {
			// If this was a bounded clerk, we should return status ok.
			status = proc.MakeStatusInfo(proc.StatusOK, "e2e time", time.Since(start))
		} else {
			// If this was an unbounded clerk, we should return status evicted.
			status = proc.MakeStatus(proc.StatusEvicted)
		}
	}
	kc.Exited(status)
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

func test(kc *kv.KvClerk, ntest uint64, p *perf.Perf, putget bool) error {
	for i := uint64(0); i < kv.NKEYS && atomic.LoadInt32(&done) == 0; i++ {
		key := kv.MkKey(i)
		v := Value{proc.GetPid(), key, ntest}
		if putget {
			// If doing puts & gets (bounded clerk)
			if err := kc.Set(key, []byte(proc.GetPid().String()), 0); err != nil {
				return fmt.Errorf("%v: Put %v err %v", proc.GetName(), key, err)
			}
			// Record op for throughput calculation.
			p.TptTick(1.0)
			if _, err := kc.Get(key, 0); err != nil {
				return fmt.Errorf("%v: Get %v err %v", proc.GetName(), key, err)
			}
			// Record op for throughput calculation.
			p.TptTick(1.0)
		} else {
			// If doing appends (unbounded clerk)
			if err := kc.AppendJson(key, v); err != nil {
				return fmt.Errorf("%v: Append %v err %v", proc.GetName(), key, err)
			}
			// Record op for throughput calculation.
			p.TptTick(1.0)
			if err := check(kc, key, ntest, p); err != nil {
				db.DPrintf(db.ALWAYS, "check failed %v\n", err)
				return err
			}
		}
	}
	return nil
}
