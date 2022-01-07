package kv

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"math/big"
	"regexp"
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

//
// Clerk for sharded kv service, which repeatedly performs key
// lookups.  The clerk acquires a lease for the configuration file of
// the balancer, which maps shards to kv groups.  If the balancer adds
// or removes a kv group, the clerk will not be able to perform
// operations at any kv group until it reacquires the configuration
// lease, bringing it up to date. The leases avoids the risk that
// clerk performs an operation at a kv group that doesn't hold the
// shard anymore (or is in the process of moving a shard)
//
// The clerk also acquires leases for each kv group it interacts with.
// If the kv group changes, the clerk cannot perform any operation at
// that kv group until it has the new configuration for that group.
// This lease avoids the risk that the clerk performs an operation at
// a KV server that isn't te primary anymore.
//

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

func key(k uint64) string {
	return "key" + strconv.FormatUint(k, 16)
}

type KvClerk struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	balLease  *leaseclnt.LeaseClnt
	grpLeases map[string]*leaseclnt.LeaseClnt
	blConf    Config
	nop       int
	grpre     *regexp.Regexp
	eofre     *regexp.Regexp
}

func MakeClerk(name string, namedAddr []string) *KvClerk {
	kc := &KvClerk{}
	kc.FsLib = fslib.MakeFsLibAddr(name, namedAddr)
	kc.balLease = leaseclnt.MakeLeaseClnt(kc.FsLib, KVCONFIG, 0)
	kc.grpLeases = make(map[string]*leaseclnt.LeaseClnt)
	kc.ProcClnt = procclnt.MakeProcClnt(kc.FsLib)
	kc.grpre = regexp.MustCompile(`name/group/grp-([0-9])-conf`)
	kc.eofre = regexp.MustCompile(`name/group/grp-([0-9])/shard`)

	err := kc.readConfig()
	if err != nil {
		log.Printf("%v: MakeClerk readConfig err %v\n", db.GetName(), err)
	}
	return kc
}

func (kc *KvClerk) waitEvict(ch chan bool) {
	err := kc.WaitEvict(proc.GetPid())
	if err != nil {
		log.Fatalf("Error WaitEvict: %v", err)
	}
	log.Printf("%v: evicted\n", db.GetName())
	ch <- true
	log.Printf("%v: evicted return\n", db.GetName())
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
	log.Printf("%v: done nop %v\n", db.GetName(), kc.nop)
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
func (kc *KvClerk) doRetry(err error, fn string) (bool, error) {
	if err.Error() == "Version mismatch" {
		log.Printf("VERSION MISMATCH\n")
	}
	if err.Error() == "EOF" || // has grp been removed from config/
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

		if err.Error() == "EOF" {
			// has balencer removed a kv group and we dont' know yet?
			s := kc.eofre.FindStringSubmatch(fn)
			if s != nil {
				// release lease, because otherwise we cannot read config
				log.Printf("%v: doRetry EOF %v\n", db.GetName(), s[1])
				err := kc.releaseLease("grp-" + s[1])
				if err != nil {
					return false, nil
				}
			}

		}

		// release lease for bal conf and reread conf?
		if strings.Contains(err.Error(), KVCONFIG) {
			log.Printf("%v: release lease %v err %v\n", db.GetName(), KVCONFIG, err)
			err := kc.balLease.ReleaseRLease()
			if err != nil {
				return false, err
			}
			err = kc.readConfig()
			if err != nil {
				log.Printf("%v: readConfig err %v\n", db.GetName(), err)
				return false, err
			}
		} else {
			// release lease for kv group?
			s := kc.grpre.FindStringSubmatch(err.Error())
			if s != nil {
				err := kc.releaseLease("grp-" + s[1])
				if err != nil {
					return false, nil
				}
			}
		}
		return true, nil
	}
	return false, err
}

func (kc *KvClerk) releaseLease(grp string) error {
	l, ok := kc.grpLeases[grp]
	if !ok {
		return fmt.Errorf("release lease %v not found", grp)
	}
	log.Printf("%v: release grp %v\n", db.GetName(), grp)
	err := l.ReleaseRLease()
	if err != nil {
		return err
	}
	delete(kc.grpLeases, grp)
	return nil
}

func (kc *KvClerk) acquireLease(grp string) error {
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

func (o *op) do(fsl *fslib.FsLib, fn string) {
	switch o.kind {
	case GET:
		o.b, o.err = fsl.GetFile(fn)
	case PUT:
		_, o.err = fsl.PutFile(fn, o.b, 0777, np.OWRITE)
	case SET:
		_, o.err = fsl.SetFile(fn, o.b)
	}
	log.Printf("%v: op %v fn %v err %v\n", db.GetName(), o.kind, fn, o.err)
}

func (kc *KvClerk) doop(o *op) {
	shard := key2shard(o.k)
	retry := true
	for retry {
		fn := keyPath(kc.blConf.Shards[shard], strconv.Itoa(shard), o.k)
		o.err = kc.acquireLease(kc.blConf.Shards[shard])
		if o.err != nil {
			// no lease on kv group (or bal conf)?
			retry, o.err = kc.doRetry(o.err, fn)
			if o.err != nil {
				return
			}
		}
		o.do(kc.FsLib, fn)
		if o.err == nil { // success?
			kc.nop += 1
			return
		}
		// invalid lease for kv group or bal conf?
		retry, o.err = kc.doRetry(o.err, fn)
		if o.err != nil {
			return
		}
	}
	log.Printf("%v: no retry %v\n", db.GetName(), o.k)
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
