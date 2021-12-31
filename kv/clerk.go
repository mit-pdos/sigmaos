package kv

import (
	"crypto/rand"
	"encoding/json"
	"hash/fnv"
	"log"
	"math/big"
	"strconv"
	"strings"
	// "time"

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
	nget  int
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
		err.Error() == "Version mismatch" ||
		strings.HasPrefix(err.Error(), "stale lease") ||
		strings.HasPrefix(err.Error(), "checkLease failed") {
		// log.Printf("doRetry error %v\n", err)
		err = kc.lease.ReleaseRLease()
		if err != nil {
			return false
		}
		return true
	}
	return false
}

func (kc *KvClerk) Set(k, v string) error {
	shard := key2shard(k)
	for {
		fn := keyPath(kc.conf.Shards[shard], strconv.Itoa(shard), k)
		// log.Printf("set %v\n", fn)
		_, err := kc.fsl.SetFile(fn, []byte(v))
		if err == nil {
			return err
		}
		db.DLPrintf("CLERK", "Set: %v %v %v\n", fn, err, shard)
		if kc.doRetry(err) {
			err = kc.readConfig()
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
}

func (kc *KvClerk) Put(k, v string) error {
	shard := key2shard(k)
	for {
		fn := keyPath(kc.conf.Shards[shard], strconv.Itoa(shard), k)
		// log.Printf("put %v\n", fn)
		_, err := kc.fsl.PutFile(fn, []byte(v), 0777, np.OWRITE)
		if err == nil {
			return err
		}
		db.DLPrintf("CLERK", "Put: %v %v %v\n", fn, err, shard)
		if kc.doRetry(err) {
			kc.readConfig()
		} else {
			return err
		}
	}
}

func (kc *KvClerk) Get(k string) (string, error) {
	shard := key2shard(k)
	for {
		fn := keyPath(kc.conf.Shards[shard], strconv.Itoa(shard), k)
		b, err := kc.fsl.GetFile(fn)
		db.DLPrintf("CLERK", "Get: %v %v\n", fn, err)
		if err == nil {
			kc.nget += 1
			return string(b), err
		}
		if kc.doRetry(err) {
			err = kc.readConfig()
			if err != nil {
				log.Printf("%v: Get readConfig err %v\n", db.GetName(), err)
				return string(b), err
			}
		} else {
			return string(b), err
		}
	}
}

func (kc *KvClerk) KVs() []string {
	err := kc.readConfig()
	if err != nil {
		log.Printf("%v: KVs readConfig err %v\n", db.GetName(), err)
	}
	kcs := makeKvs(kc.conf.Shards)
	return kcs.mkKvs()
}
