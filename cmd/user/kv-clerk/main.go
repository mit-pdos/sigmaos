package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync/atomic"

	"ulambda/fslib"
	"ulambda/kv"
	np "ulambda/ninep"
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
		log.Printf("Error WaitEvict: %v", err)
	}
	log.Printf("%v: Evict\n", proc.GetName())
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
	b, err := kc.Get(kv.Key(i), 0)
	if err != nil {
		return err
	}
	rdr := bytes.NewReader(b)
	for {
		l, err := binary.ReadVarint(rdr)
		if err != nil && err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("rdr err %v\n", err)
			return err
		}
		v := make([]byte, np.Tsize(l))
		if _, err := rdr.Read(v); err != nil {
			log.Printf("rdr val err %v\n", err)
			return err
		}
		val := Value{}
		if err := json.Unmarshal(v, &val); err != nil {
			log.Printf("unmarshal err %v\n", err)
			return err
		}
		if val.Pid != proc.GetPid() {
			continue
		}
		if val.Key != kv.Key(i) {
			return fmt.Errorf("%v: wrong key %v %v", proc.GetName(), kv.Key(i), val.Key)
		}
		if val.N != n {
			return fmt.Errorf("%v: wrong N %v %v", proc.GetName(), n, val.N)
		}
		n += 1
	}
	if n < ntest {
		return fmt.Errorf("%v: wrong ntest %v %v", proc.GetName(), ntest, n)
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
			return err
		}
	}
	return nil
}
