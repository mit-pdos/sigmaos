package kv

import (
	"crypto/rand"
	"encoding/json"
	"hash/fnv"
	"log"
	"math/big"
	"strconv"
	"strings"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/leaseclnt"
	np "ulambda/ninep"
	"ulambda/proc"
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
	fsl   *fslib.FsLib
	lease *leaseclnt.LeaseClnt
	conf  Config
	nop   int
}

func MakeClerk(namedAddr []string) *KvClerk {
	kc := &KvClerk{}
	kc.fsl = fslib.MakeFsLibAddr("clerk-"+proc.GetPid(), namedAddr)
	kc.lease = leaseclnt.MakeLeaseClnt(kc.fsl, KVCONFIG, 0)
	err := kc.readConfig()
	if err != nil {
		log.Printf("%v: MakeClerk readConfig err %v\n", db.GetName(), err)
	}
	return kc
}

func (kc *KvClerk) Exit() {
	kc.fsl.Exit()
}

// XXX atomic read
func (kc *KvClerk) readConfig() error {
	b, err := kc.lease.WaitRLease()
	if err != nil {
		log.Printf("%v: clerk readConfig: err %v\n", db.GetName(), err)
		return err
	}
	json.Unmarshal(b, &kc.conf)
	// log.Printf("%v: readConfig %v\n", db.GetName(), kc.conf)
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
		err = kc.lease.ReleaseRLease()
		if err != nil {
			return false
		}
		return true
	}
	return false
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
		fn := keyPath(kc.conf.Shards[shard], strconv.Itoa(shard), o.k)
		switch o.kind {
		case GET:
			o.b, o.err = kc.fsl.GetFile(fn)
		case PUT:
			_, o.err = kc.fsl.PutFile(fn, o.b, 0777, np.OWRITE)
		case SET:
			_, o.err = kc.fsl.SetFile(fn, o.b)
		}
		db.DLPrintf("CLERK", "%v: %v %v\n", o.kind, fn, o.err)
		if o.err == nil {
			kc.nop += 1
			return
		}
		if kc.doRetry(o.err) {
			o.err = kc.readConfig()
			if o.err != nil {
				log.Printf("%v: %v readConfig err %v\n", db.GetName(), o.kind, o.err)
				return
			}
		} else {
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
	kcs := makeKvs(kc.conf.Shards)
	return kcs.mkKvs()
}
