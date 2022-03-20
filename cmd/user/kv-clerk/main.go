package main

import (
	"fmt"
	"log"
	"os"
	"sync/atomic"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/kv"
	"ulambda/proc"
)

var done = int32(0)

func main() {
	if len(os.Args) < 1 {
		fmt.Fprintf(os.Stderr, "%v\n", os.Args[0])
		os.Exit(1)
	}
	clk, err := kv.MakeClerk("clerk-"+proc.GetPid(), fslib.Named())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v err %v\n", os.Args[0], err)
		os.Exit(1)
	}
	clk.Started(proc.GetPid())
	run(clk)
}

func waitEvict(kc *kv.KvClerk) {
	err := kc.WaitEvict(proc.GetPid())
	if err != nil {
		db.DLPrintf("KVCLERK", "Error WaitEvict: %v", err)
	}
	db.DLPrintf("KVCLERK", "Evict\n")
	atomic.StoreInt32(&done, 1)
}

func run(kc *kv.KvClerk) {
	ntest := uint64(0)
	var err error
	go waitEvict(kc)
	for atomic.LoadInt32(&done) == 0 {
		err = test(kc, ntest)
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
	kc.Exited(proc.GetPid(), status)
}

type Value struct {
	Pid string
	Key kv.Tkey
	N   uint64
}

func check(kc *kv.KvClerk, key kv.Tkey, ntest uint64) error {
	n := uint64(0)
	rdr, err := kc.GetReader(key)
	if err != nil {
		return err
	}
	rdr.Unfence()
	defer rdr.Close()
	err = rdr.ReadJsonStream(func() interface{} { return new(Value) }, func(a interface{}) error {
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
		log.Printf("ReadJsonStream: err %v\n", err)
	}
	if n < ntest {
		return fmt.Errorf("%v: wrong ntest for %v: expected %v observed %v", proc.GetName(), rdr.Path(), ntest, n)
	}
	return nil
}

func test(kc *kv.KvClerk, ntest uint64) error {
	for i := uint64(0); i < kv.NKEYS && atomic.LoadInt32(&done) == 0; i++ {
		key := kv.MkKey(i)
		v := Value{proc.GetPid(), key, ntest}
		if err := kc.AppendJson(key, v); err != nil {
			return fmt.Errorf("%v: Append %v err %v", proc.GetName(), key, err)
		}
		if err := check(kc, key, ntest); err != nil {
			log.Printf("check failed %v\n", err)
			return err
		}
	}
	return nil
}
