package kv

import (
	"crypto/rand"
	"fmt"
	"hash/fnv"
	"log"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ulambda/fenceclnt"
	"ulambda/fslib"
	"ulambda/group"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
)

//
// Clerk for sharded kv service, which repeatedly performs key
// lookups.  The clerk acquires a fence for the configuration file of
// the balancer, which maps shards to kv groups.  If the balancer adds
// or removes a kv group, the clerk will not be able to perform
// operations at any kv group until it reacquires the configuration
// fence, bringing it up to date. The fences avoids the risk that
// clerk performs an operation at a kv group that doesn't hold the
// shard anymore (or is in the process of moving a shard)
//
// The clerk also acquires fences for each kv group it interacts with.
// If the kv group changes, the clerk cannot perform any operation at
// that kv group until it has the new configuration for that group.
// This fence avoids the risk that the clerk performs an operation at
// a KV server that isn't te primary anymore.
//

const (
	NKEYS  = 100
	WAITMS = 50
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
	balFclnt  *fenceclnt.FenceClnt
	grpFclnts map[string]*fenceclnt.FenceClnt
	blConf    Config
	nop       int
	grpre     *regexp.Regexp
}

func MakeClerk(name string, namedAddr []string) *KvClerk {
	kc := &KvClerk{}
	kc.FsLib = fslib.MakeFsLibAddr(name, namedAddr)
	kc.balFclnt = fenceclnt.MakeFenceClnt(kc.FsLib, KVCONFIG, 0)
	kc.grpFclnts = make(map[string]*fenceclnt.FenceClnt)
	kc.ProcClnt = procclnt.MakeProcClnt(kc.FsLib)
	kc.grpre = regexp.MustCompile(`group/grp-([0-9]+)-conf`)
	err := kc.balFclnt.AcquireConfig(&kc.blConf)
	if err != nil {
		log.Printf("%v: MakeClerk readConfig err %v\n", proc.GetProgram(), err)
	}
	return kc
}

func (kc *KvClerk) waitEvict(ch chan bool) {
	err := kc.WaitEvict(proc.GetPid())
	if err != nil {
		log.Printf("Error WaitEvict: %v", err)
	}
	ch <- true
}

func (kc *KvClerk) getKeys(ch chan bool) (bool, error) {
	for i := uint64(0); i < NKEYS; i++ {
		v, err := kc.Get(key(i))
		select {
		case <-ch:
			// ignore error from Get()
			return true, nil
		default:
			if err != nil {
				return false, fmt.Errorf("%v: Get %v err %v", proc.GetProgram(), key(i), err)
			}
			if key(i) != v {
				return false, fmt.Errorf("%v: Get %v wrong val %v", proc.GetProgram(), key(i), v)
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
			break
		}
	}
	log.Printf("%v: done nop %v done %v err %v\n", proc.GetProgram(), kc.nop, done, err)
	s := "OK"
	if err != nil {
		s = err.Error()
	}

	// We want exited() to not fail because of invalid fences
	// (e.g., we may still have a fence for a kv group that
	// doesn't exist anymore. Since we don't need fences anymore,
	// deregister the ones we have.
	kc.DeregisterFences()
	kc.Exited(proc.GetPid(), s)
}

func (kc *KvClerk) releaseFence(grp string) error {
	f, ok := kc.grpFclnts[grp]
	if !ok {
		return fmt.Errorf("release fclnt %v not found", grp)
	}
	// log.Printf("%v: release grp %v\n", proc.GetProgram(), grp)
	err := f.ReleaseFence()
	if err != nil {
		return err
	}
	return nil
}

// Dynamically allocate a FenceClnt if we haven't seen this grp before.
func (kc *KvClerk) acquireFence(grp string) error {
	if fc, ok := kc.grpFclnts[grp]; ok {
		if fc.IsFenced() != nil {
			// we have acquired a fence
			return nil
		}
	} else {
		fn := group.GrpConfPath(grp)
		kc.grpFclnts[grp] = fenceclnt.MakeFenceClnt(kc.FsLib, fn, 0)
	}
	gc := group.GrpConf{}
	err := kc.grpFclnts[grp].AcquireConfig(&gc)
	if err != nil {
		return err
	}
	// XXX do something with gc
	return nil
}

// Try fix err by releasing group fence
func (kc KvClerk) releaseGrp(err error) error {
	s := kc.grpre.FindStringSubmatch(err.Error())
	if s != nil {
		return kc.releaseFence("grp-" + s[1])
	}
	return err
}

// Read config, and retry if we have a stale group fence
func (kc KvClerk) retryReadConfig() error {
	for {
		err := kc.balFclnt.AcquireConfig(&kc.blConf)
		if err == nil {
			return nil
		}
		err = kc.releaseGrp(err)
		if err == nil {
			log.Printf("%v: retry readConfig\n", proc.GetProgram())
			continue
		}

		// maybe retryReadConfig failed with a stale error
		if strings.HasPrefix(err.Error(), "stale") {
			log.Printf("%v: retry refreshConfig %v\n", proc.GetProgram(), err)
			continue
		}

		// bail out; some other problem
		return err
	}
}

// Try to fix err by refreshing config fence and rereading it
func (kc KvClerk) refreshConfig(err error) error {
	for {
		// refence fence, just in case we will still have one
		kc.balFclnt.ReleaseFence()

		err = kc.retryReadConfig()
		if err == nil {
			return nil
		}

		if strings.Contains(err.Error(), "not found config") {
			log.Printf("%v: retry refreshConfig %v\n", proc.GetProgram(), err)
			continue
		}

		// maybe we couldn't get fence or read config because
		// we have stale grp fence; check and retry if so.
		err = kc.releaseGrp(err)
		if err == nil {
			log.Printf("%v: retry refreshConfig\n", proc.GetProgram())
			continue
		}

		// bail out; some other problem
		return err
	}
}

// Try to fix err by refreshing fences
func (kc *KvClerk) refreshFences(err error) error {
	// first check if refreshing group fence is sufficient to retry
	err = kc.releaseGrp(err)
	if err != nil {
		// try refreshing config is sufficient to fix error
		// involving KVCONFIG or if EOF to a kv group.
		if strings.Contains(err.Error(), KVCONFIG) ||
			strings.HasPrefix(err.Error(), "stale") ||
			err.Error() == "EOF" {
			err = kc.refreshConfig(err)
		}
	}
	return err
}

// Try to fix err; if return is nil, retry.
func (kc *KvClerk) fixRetry(err error) error {
	// log.Printf("%v: fixRetry err %v\n", proc.GetProgram(), err)

	// Shard dir hasn't been created yet (config 0) or hasn't moved
	// yet, so wait a bit, and retry.  XXX make sleep time
	// dynamic?
	if strings.HasPrefix(err.Error(), "file not found shard") {
		time.Sleep(WAITMS * time.Millisecond)
		return nil
	}

	// Maybe refreshing fences will help in fixing error
	return kc.refreshFences(err)
}

// Do an operation, but it may fail for several reasons (stale config
// fence, stale group leae). If an error, try to fix the error, and on
// success, retry.
func (kc *KvClerk) doop(o *op) {
	shard := key2shard(o.k)
	for {
		fn := keyPath(kc.blConf.Shards[shard], strconv.Itoa(shard), o.k)
		o.err = kc.acquireFence(kc.blConf.Shards[shard])
		if o.err != nil {
			o.err = kc.fixRetry(o.err)
			if o.err != nil {
				return
			}
			// try again to acquire group fence
			continue
		}
		o.do(kc.FsLib, fn)
		if o.err == nil { // success?
			kc.nop += 1
			return
		}
		o.err = kc.fixRetry(o.err)
		if o.err != nil {
			return
		}
	}
	//log.Printf("%v: no retry %v\n", proc.GetProgram(), o.k)
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
	// log.Printf("%v: op %v fn %v err %v\n", proc.GetProgram(), o.kind, fn, o.err)
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
