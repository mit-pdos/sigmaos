package main

import (
	"fmt"
	"log"
	"os"
	"sync/atomic"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/kv"
	"ulambda/perf"
	"ulambda/proc"
)

var done = int32(0)

func main() {
	if len(os.Args) < 1 {
		fmt.Fprintf(os.Stderr, "%v\n", os.Args[0])
		os.Exit(1)
	}
	clk, err := kv.MakeClerk("clerk-"+proc.GetPid().String(), fslib.Named())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v err %v\n", os.Args[0], err)
		os.Exit(1)
	}
	// Record performance.
	p := perf.MakePerf("KVCLERK")
	defer p.Done()
	clk.Started()
	run(clk, p)
}

func waitEvict(kc *kv.KvClerk) {
	err := kc.WaitEvict(proc.GetPid())
	if err != nil {
		db.DPrintf("KVCLERK", "Error WaitEvict: %v", err)
	}
	db.DPrintf("KVCLERK", "Evict\n")
	atomic.StoreInt32(&done, 1)
}

func run(kc *kv.KvClerk, p *perf.Perf) {
	ntest := uint64(0)
	var err error
	go waitEvict(kc)
	for atomic.LoadInt32(&done) == 0 {
		err = test(kc, ntest, p)
		if err != nil {
			break
		}
		ntest += 1
	}
	log.Printf("%v: done ntest %v err %v\n", proc.GetName(), ntest, err)
	var status *proc.Status
	if err != nil {
		status = proc.MakeStatusErr(err.Error(), nil)
	} else {
		status = proc.MakeStatus(proc.StatusOK)
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
		log.Printf("JsonReader: err %v\n", err)
	}
	if n < ntest {
		return fmt.Errorf("%v: wrong ntest for %v: expected %v observed %v", proc.GetName(), rdr.Path(), ntest, n)
	}
	return nil
}

func test(kc *kv.KvClerk, ntest uint64, p *perf.Perf) error {
	for i := uint64(0); i < kv.NKEYS && atomic.LoadInt32(&done) == 0; i++ {
		key := kv.MkKey(i)
		v := Value{proc.GetPid(), key, ntest}
		if err := kc.AppendJson(key, v); err != nil {
			return fmt.Errorf("%v: Append %v err %v", proc.GetName(), key, err)
		}
		// Record op for throughput calculation.
		p.TptTick(1.0)
		if err := check(kc, key, ntest, p); err != nil {
			log.Printf("check failed %v\n", err)
			return err
		}
	}
	return nil
}
