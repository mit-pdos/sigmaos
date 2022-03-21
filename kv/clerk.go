package kv

import (
	"crypto/rand"
	"fmt"
	"hash/fnv"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	db "ulambda/debug"
	"ulambda/fenceclnt"
	"ulambda/fslib"
	"ulambda/group"
	np "ulambda/ninep"
	"ulambda/procclnt"
	"ulambda/reader"
	"ulambda/writer"
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

func Key(k uint64) string {
	return "key" + strconv.FormatUint(k, 16)
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
	balFclnt  *fenceclnt.FenceClnt
	grpFclnts map[string]*fenceclnt.FenceClnt
	blConf    Config
	grpre     *regexp.Regexp
	grpconfre *regexp.Regexp
}

func MakeClerk(name string, namedAddr []string) *KvClerk {
	kc := &KvClerk{}
	kc.FsLib = fslib.MakeFsLibAddr(name, namedAddr)
	kc.balFclnt = fenceclnt.MakeFenceClnt(kc.FsLib, KVCONFIG, 0, []string{KVDIR})
	kc.grpFclnts = make(map[string]*fenceclnt.FenceClnt)
	kc.ProcClnt = procclnt.MakeProcClnt(kc.FsLib)
	kc.grpconfre = regexp.MustCompile(`group/grp-([0-9]+)-conf`)
	kc.grpre = regexp.MustCompile(`grp-([0-9]+)`)
	err := kc.balFclnt.AcquireConfig(&kc.blConf)
	if err != nil {
		db.DLPrintf("KVCLERK", "MakeClerk readConfig err %v\n", err)
	}
	return kc
}

func (kc *KvClerk) releaseFence(grp string) error {
	f, ok := kc.grpFclnts[grp]
	if !ok {
		return fmt.Errorf("release fclnt %v not found", grp)
	}
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
		paths := []string{group.GrpDir(grp)}
		kc.grpFclnts[grp] = fenceclnt.MakeFenceClnt(kc.FsLib, fn, 0, paths)
		// Fence new group also with balancer config fence
		kc.balFclnt.FencePaths(paths)
	}
	gc := group.GrpConf{}
	err := kc.grpFclnts[grp].AcquireConfig(&gc)
	if err != nil {
		return err
	}
	// XXX do something with gc
	return nil
}

// Remove group from fenced paths in bal fclnt?
func (kc KvClerk) removeGrp(err error) error {
	if np.IsErrUnreachable(err) {
		s := kc.grpre.FindStringSubmatch(np.ErrPath(err))
		if s != nil {
			if kvs, r := readKVs(kc.FsLib); r == nil {
				if !kvs.present(s[0]) {
					if _, ok := kc.grpFclnts[s[0]]; ok {
						delete(kc.grpFclnts, s[0])
					}
					paths := []string{group.GrpDir(s[0])}
					kc.balFclnt.RemovePaths(paths)
					if r == nil {
						return nil
					}
				}
			} else {
				db.DLPrintf("KVCLERK", "ReadFileJson %v err %v\n", KVCONFIG, r)
				return r
			}
		}
	}
	return err
}

// Try fix err by releasing group fence
func (kc KvClerk) releaseGrp(err error) error {
	s := kc.grpconfre.FindStringSubmatch(err.Error())
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
		err = kc.removeGrp(err)
		if err == nil {
			db.DLPrintf("KVCLERK", "removeGrp readConfig\n")
			continue
		}
		err = kc.releaseGrp(err)
		if err == nil {
			db.DLPrintf("KVCLERK", "releaseGrp readConfig\n")
			continue
		}

		// maybe retryReadConfig failed with a stale error
		if np.IsErrStale(err) {
			db.DLPrintf("KVCLERK", "stale readConfig %v\n", err)
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

		if np.IsErrUnreachable(err) && strings.Contains(np.ErrPath(err), KVCONF) {
			db.DLPrintf("KVCLERK", "retry refreshConfig %v\n", err)
			continue
		}

		// maybe we couldn't get fence or read config because
		// we have stale grp fence; check and retry if so.
		err = kc.releaseGrp(err)
		if err == nil {
			db.DLPrintf("KVCLERK", "retry refreshConfig\n")
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
		if np.IsErrUnreachable(err) || np.IsErrStale(err) {
			err = kc.refreshConfig(err)
		}
	}
	return err
}

// Try to fix err; if return is nil, retry.
func (kc *KvClerk) fixRetry(err error) error {

	// Shard dir hasn't been created yet (config 0) or hasn't moved
	// yet, so wait a bit, and retry.  XXX make sleep time
	// dynamic?

	if np.IsErrNotfound(err) && strings.HasPrefix(np.ErrPath(err), "shard") {
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
			return
		}
		o.err = kc.fixRetry(o.err)
		if o.err != nil {
			return
		}
	}
}

type opT int

const (
	GETVAL opT = iota + 1
	PUT
	SET
	GETRD
)

type op struct {
	kind opT
	b    []byte
	k    string
	off  np.Toffset
	rdr  *reader.Reader
	err  error
}

func (o *op) do(fsl *fslib.FsLib, fn string) {
	switch o.kind {
	case GETVAL:
		o.b, o.err = fsl.GetFile(fn)
	case GETRD:
		o.rdr, o.err = fsl.OpenReader(fn)
	case PUT:
		_, o.err = fsl.PutFile(fn, 0777, np.OWRITE, o.b)
	case SET:
		_, o.err = fsl.SetFile(fn, o.b, o.off)
	}
	db.DLPrintf("KVCLERK", "op %v fn %v err %v\n", o.kind, fn, o.err)
}

func (kc *KvClerk) Get(k string, off np.Toffset) ([]byte, error) {
	op := &op{GETVAL, []byte{}, k, off, nil, nil}
	kc.doop(op)
	return op.b, op.err
}

func (kc *KvClerk) GetReader(k string) (*reader.Reader, error) {
	op := &op{GETRD, []byte{}, k, 0, nil, nil}
	kc.doop(op)
	return op.rdr, op.err
}

func (kc *KvClerk) Set(k string, b []byte, off np.Toffset) error {
	op := &op{SET, b, k, off, nil, nil}
	kc.doop(op)
	return op.err
}

func (kc *KvClerk) Append(k string, b []byte) error {
	op := &op{SET, b, k, np.NoOffset, nil, nil}
	kc.doop(op)
	return op.err
}

func (kc *KvClerk) Put(k string, b []byte) error {
	op := &op{PUT, b, k, 0, nil, nil}
	kc.doop(op)
	return op.err
}

func (kc *KvClerk) AppendJson(k string, v interface{}) error {
	b, err := writer.JsonRecord(v)
	if err != nil {
		return err
	}
	op := &op{SET, b, k, np.NoOffset, nil, nil}
	kc.doop(op)
	return op.err
}
