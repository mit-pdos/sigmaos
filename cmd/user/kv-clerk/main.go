package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
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
	clk := kv.MakeClerk("clerk-"+proc.GetPid(), fslib.Named())
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
	Key string
	N   uint64
}

func check(kc *kv.KvClerk, i, ntest uint64) error {
	n := uint64(0)
	rdr, err := kc.GetReader(kv.Key(i))
	if err != nil {
		return err
	}
	defer rdr.Close()
	for {
		sz, err := binary.ReadVarint(rdr)
		if err != nil && err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("%v: ReadVarint err %v", proc.GetName(), err)
		}
		data := make([]byte, sz)
		l, err := rdr.Read(data)
		if l != len(data) {
			log.Fatalf("FATAL %v missing data %v %v\n", proc.GetName(), l, len(data))
		}
		if err != nil {
			log.Printf("%v: Read err %v\n", proc.GetName(), err)
			return err
		}
		val := Value{}
		if err := json.Unmarshal(data, &val); err != nil {
			log.Printf("%v: unmarshal err %v\n", proc.GetName(), err)
			return err
		}
		if val.Pid != proc.GetPid() {
			continue
		}
		if val.Key != kv.Key(i) {
			return fmt.Errorf("%v: wrong key %v %v %v", proc.GetName(), kv.Key(i), val.Key, kv.Key(i))
		}
		if val.N != n {
			return fmt.Errorf("%v: wrong N %v %v %v %v", proc.GetName(), n, val.N, kv.Key(i), rdr.Path())
		}
		n += 1
	}
	if n < ntest {
		return fmt.Errorf("%v: wrong ntest %v %v %v %v", proc.GetName(), ntest, n, kv.Key(i), rdr.Path())
	}
	return nil
}

func test(kc *kv.KvClerk, ntest uint64) error {
	for i := uint64(0); i < kv.NKEYS && atomic.LoadInt32(&done) == 0; i++ {
		v := Value{proc.GetPid(), kv.Key(i), ntest}
		if err := kc.AppendJson(kv.Key(i), v); err != nil {
			return fmt.Errorf("%v: Append %v err %v", proc.GetName(), kv.Key(i), err)
		}
		if err := check(kc, i, ntest); err != nil {
			log.Printf("check failed %v\n", err)
			return err
		}
	}
	return nil
}
