package kv

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"math/big"
	"strconv"
	"strings"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/group"
	"ulambda/leaseclnt"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
)

const (
	NKEYS = 100
)

func key2shard(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	shard := int(h.Sum32() % NSHARD)
	return shard
}

func keyPath(kvd, shard string, k string) string {
	d := shardPath(kvd, shard)
	return d + "/" + k
}

func nrand() uint64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Uint64()
	return x
}

type KvClerk struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	balLease  *leaseclnt.LeaseClnt
	grpLeases map[string]*leaseclnt.LeaseClnt
	blConf    Config
	nop       int
}

func MakeClerk(name string, namedAddr []string) *KvClerk {
	kc := &KvClerk{}
	kc.FsLib = fslib.MakeFsLibAddr(name, namedAddr)
	kc.balLease = leaseclnt.MakeLeaseClnt(kc.FsLib, KVCONFIG, 0)
	kc.grpLeases = make(map[string]*leaseclnt.LeaseClnt)
	kc.ProcClnt = procclnt.MakeProcClnt(kc.FsLib)
	err := kc.readConfig()
	if err != nil {
		log.Printf("%v: MakeClerk readConfig err %v\n", db.GetName(), err)
	}
	return kc
}

func key(k uint64) string {
	return "key" + strconv.FormatUint(k, 16)
}

func (kc *KvClerk) getKeys(ch chan bool) (bool, error) {
	for i := uint64(0); i < NKEYS; i++ {
		v, err := kc.Get(key(i))
		select {
		case <-ch:
			return true, nil
		default:
			if err != nil {
				return false, fmt.Errorf("%v: Get %v err %v\n", db.GetName(), key(i), err)
			}
			if key(i) != v {
				return false, fmt.Errorf("%v: Get %v wrong val %v\n", db.GetName(), key(i), v)
			}
		}
	}
	return false, nil
}

func (kc *KvClerk) waitEvict(ch chan bool) {
	err := kc.WaitEvict(proc.GetPid())
	if err != nil {
		log.Fatalf("Error WaitEvict: %v", err)
	}
	ch <- true
}

func (kc *KvClerk) Run() {
	kc.Started(proc.GetPid())
	ch := make(chan bool)
	done := false
	var err error
	go kc.waitEvict(ch)
	for !done {
		done, err = kc.getKeys(ch)
		if err != nil {
			kc.Exited(proc.GetPid(), err.Error())
		}
	}
	log.Printf("nop %v\n", kc.nop)
	kc.Exited(proc.GetPid(), "OK")
}

// XXX atomic read
func (kc *KvClerk) readConfig() error {
	b, err := kc.balLease.WaitRLease()
	if err != nil {
		log.Printf("%v: clerk readConfig: err %v\n", db.GetName(), err)
		return err
	}
	json.Unmarshal(b, &kc.blConf)
	log.Printf("%v: readConfig %v\n", db.GetName(), kc.blConf)
	return nil
}

// XXX error checking in one place and more uniform
func (kc *KvClerk) doRetry(err error) bool {
	if err.Error() == "Version mismatch" {
		log.Printf("VERSION MISMATCH\n")
	}

	if err.Error() == "EOF" || // XXX maybe useful when KVs fail
		// XXX unused?
		err.Error() == "Version mismatch" ||
		// shard dir hasn't been create yet (config 0) or hasn't
		// moved yet
		strings.HasPrefix(err.Error(), "file not found shard") ||
		// lease ran out
		strings.HasPrefix(err.Error(), "stale lease") ||
		// lease ran out  XXX one error?
		strings.HasPrefix(err.Error(), "lease not found") {
		// log.Printf("doRetry error %v\n", err)

		// XXX release grp lease for certain errors
		// err = kc.balLease.ReleaseRLease()

		err = kc.balLease.ReleaseRLease()
		if err != nil {
			return false
		}
		return true
	}
	return false
}

func (kc *KvClerk) lease(grp string) error {
	gc := group.GrpConf{}
	if _, ok := kc.grpLeases[grp]; ok {
		return nil
	}
	fn := group.GrpConfPath(grp)
	l := leaseclnt.MakeLeaseClnt(kc.FsLib, fn, 0)
	b, err := l.WaitRLease()
	if err != nil {
		log.Printf("%v: lease %v err %v\n", db.GetName(), grp, err)
		return err
	}
	kc.grpLeases[grp] = l
	json.Unmarshal(b, &gc)
	log.Printf("%v: grp lease %v gc %v\n", db.GetName(), grp, gc)
	return nil
}

type opT int

const (
	GET opT = 0
	PUT opT = 1
	SET opT = 2
)

type op struct {
	kind opT
	b    []byte
	k    string
	err  error
}

func (kc *KvClerk) doop(o *op) {
	shard := key2shard(o.k)
	for {
		o.err = kc.lease(kc.blConf.Shards[shard])
		if o.err != nil {
			// XXX what if info is wrong; kvgroup has been removed
			// readConfig() and retry
			log.Printf("%v: kc.lease %v err %v\n", db.GetName(), kc.blConf.Shards[shard], o.err)
			continue
		}
		fn := keyPath(kc.blConf.Shards[shard], strconv.Itoa(shard), o.k)
		switch o.kind {
		case GET:
			o.b, o.err = kc.GetFile(fn)
		case PUT:
			_, o.err = kc.PutFile(fn, o.b, 0777, np.OWRITE)
		case SET:
			_, o.err = kc.SetFile(fn, o.b)
		}
		log.Printf("%v: op %v fn %v err %v\n", db.GetName(), o.kind, fn, o.err)
		if o.err == nil {
			kc.nop += 1
			return
		}
		if kc.doRetry(o.err) {
			log.Printf("%v: retry %v\n", db.GetName(), o.err)
			// XXX never get Wrelease; already have it.
			o.err = kc.readConfig()
			if o.err != nil {
				log.Printf("%v: %v readConfig err %v\n", db.GetName(), o.kind, o.err)
				return
			}
			log.Printf("%v: retry now\n", db.GetName())
		} else {
			log.Printf("%v: no retry\n", db.GetName())
			return
		}
	}
}

func (kc *KvClerk) Get(k string) (string, error) {
	op := &op{GET, []byte{}, k, nil}
	kc.doop(op)
	return string(op.b), op.err
}

func (kc *KvClerk) Set(k, v string) error {
	op := &op{SET, []byte(v), k, nil}
	kc.doop(op)
	return op.err
}

func (kc *KvClerk) Put(k, v string) error {
	op := &op{PUT, []byte(v), k, nil}
	kc.doop(op)
	return op.err
}

func (kc *KvClerk) KVs() []string {
	err := kc.readConfig()
	if err != nil {
		log.Printf("%v: KVs readConfig err %v\n", db.GetName(), err)
	}
	kcs := makeKvs(kc.blConf.Shards)
	return kcs.mkKvs()
}
