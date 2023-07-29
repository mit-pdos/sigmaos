package kv

import (
	"crypto/rand"
	"errors"
	"fmt"
	"hash/fnv"
	"math/big"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/fenceclnt"
	"sigmaos/fslib"
	"sigmaos/group"
	"sigmaos/reader"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	tproto "sigmaos/tracing/proto"
)

//
// Clerk for sharded kv service, which repeatedly reads/writes keys.
//

const (
	NKEYS  = 100
	WAITMS = 50
)

type Tkey string

func (k Tkey) String() string {
	return string(k)
}

func MkKey(k uint64) string {
	return strconv.FormatUint(k, 16)
}

type Tshard int

func key2shard(key Tkey) Tshard {
	h := fnv.New32a()
	h.Write([]byte(key))
	shard := Tshard(h.Sum32() % NSHARD)
	return shard
}

func (s Tshard) String() string {
	return fmt.Sprintf("%03d", s)
}

func keyPath(job, kvd string, shard Tshard, k Tkey) string {
	d := kvShardPath(job, kvd, shard)
	return d + "/" + k.String()
}

func nrand() uint64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Uint64()
	return x
}

type KvClerk struct {
	*fslib.FsLib
	fclnt *fenceclnt.FenceClnt
	conf  *Config
	job   string
	cclnt *CacheClnt
}

func MakeClerkFsl(fsl *fslib.FsLib, job string) (*KvClerk, error) {
	return makeClerkStart(fsl, job)
}

func MakeClerkFslOnly(fsl *fslib.FsLib, job string) *KvClerk {
	return makeClerk(fsl, job)
}

func MakeClerk(uname sp.Tuname, job string) (*KvClerk, error) {
	fsl, err := fslib.MakeFsLib(uname)
	if err != nil {
		return nil, err
	}
	return makeClerkStart(fsl, job)
}

func makeClerk(fsl *fslib.FsLib, job string) *KvClerk {
	kc := &KvClerk{
		FsLib: fsl,
		conf:  &Config{},
		job:   job,
		fclnt: fenceclnt.MakeFenceClnt(fsl),
		cclnt: NewCacheClnt(fsl, NSHARD),
	}
	return kc
}

func makeClerkStart(fsl *fslib.FsLib, job string) (*KvClerk, error) {
	kc := makeClerk(fsl, job)
	return kc, kc.StartClerk()
}

func (kc *KvClerk) StartClerk() error {
	if err := kc.switchConfig(); err != nil {
		return err
	}
	return nil
}

func (kc *KvClerk) IsMiss(err error) bool {
	db.DPrintf(db.KVCLERK, "IsMiss err %v", err)
	return serr.IsErrCode(err, serr.TErrNotfound)
}

// Detach servers not in kvs
func (kc *KvClerk) DetachKVs(kvs *KvSet) {
	mnts := kc.Mounts()
	for _, mnt := range mnts {
		db.DPrintf(db.KVCLERK, "mnt kv %v", mnt)
		if strings.HasPrefix(mnt, JobDir(kc.job)+"/grp") {
			kvd := strings.TrimPrefix(mnt, JobDir(kc.job)+"/")
			if !kvs.present(kvd) {
				db.DPrintf(db.KVCLERK, "Detach kv %v", kvd)
				kc.Detach(kvGrpPath(kc.job, kvd))
			}
		}
	}
}

func paths(job string, kvset *KvSet) []string {
	kvs := kvset.mkKvs()
	dirs := make([]string, 0, len(kvs)+1)
	for _, kvd := range kvs {
		dirs = append(dirs, group.GrpPath(JobDir(job), kvd))
	}
	return dirs
}

// Read config, and retry if we have a stale group fence
func (kc *KvClerk) switchConfig() error {
	for {
		err := kc.GetFileJsonWatch(KVConfig(kc.job), kc.conf)
		if err != nil {
			db.DPrintf(db.KVCLERK_ERR, "GetFileJsonWatch %v err %v", KVConfig(kc.job), err)
			return err
		}
		db.DPrintf(db.KVCLERK, "Conf %v", kc.conf)
		kvset := MakeKvs(kc.conf.Shards)
		dirs := paths(kc.job, kvset)
		if err := kc.fclnt.FenceAtEpoch(kc.conf.Fence, dirs); err != nil {
			var serr *serr.Err
			if errors.As(err, &serr) && (serr.IsErrVersion() || serr.IsErrStale()) {
				db.DPrintf(db.KVCLERK_ERR, "version mismatch; retry")
				time.Sleep(WAITMS * time.Millisecond)
				continue
			}
			db.DPrintf(db.KVCLERK_ERR, "FenceAtEpoch %v failed %v", dirs, err)
			return err
		}

		// detach groups not in use; diff between new and mount table?
		kc.DetachKVs(kvset)
		break
	}
	return nil
}

// Try to fix err; if return is nil, retry.
func (kc *KvClerk) fixRetry(err error) error {
	var sr *serr.Err
	if errors.As(err, &sr) && sr.IsErrNotfound() && strings.HasPrefix(sr.ErrPath(), "shard") {
		// Shard dir hasn't been created yet (config 0) or hasn't moved
		// yet, so wait a bit, and retry.  XXX make sleep time
		// dynamic?
		db.DPrintf(db.KVCLERK_ERR, "Wait for shard %v", sr.ErrPath())
		time.Sleep(WAITMS * time.Millisecond)
		return nil
	}
	if serr.IsErrCode(err, serr.TErrStale) {
		db.DPrintf(db.KVCLERK_ERR, "fixRetry %v", err)
		return kc.switchConfig()
	}
	return err
}

// Do an operation. If an error, try to fix the error (e.g., rereading
// config), and on success, retry.
func (kc *KvClerk) doop(o *op) {
	s := key2shard(o.k)
	for {
		db.DPrintf(db.KVCLERK, "o %v conf %v", o.kind, kc.conf)
		fn := keyPath(kc.job, kc.conf.Shards[s], s, o.k)
		kc.do(o, fn, kvGrpPath(kc.job, kc.conf.Shards[s]), s)
		if o.err == nil { // success?
			return
		}
		o.err = kc.fixRetry(o.err)
		if o.err != nil {
			return
		}
	}
}

type opT string

const (
	GETVAL = "Get"
	PUT    = "Put"
	GETRD  = "Read"
)

type op struct {
	kind opT
	val  proto.Message
	k    Tkey
	off  sp.Toffset
	m    sp.Tmode
	rdr  *reader.Reader
	err  error
}

func (kc *KvClerk) do(o *op, fn string, srv string, s Tshard) {
	switch o.kind {
	case GETVAL:
		o.err = kc.cclnt.Get(srv, string(o.k), o.val)
		// o.b, o.err = fsl.GetFile(fn)
	case GETRD:
		db.DFatalf("do not supported\n")
		// o.rdr, o.err = fsl.OpenReader(fn)
	case PUT:
		o.err = kc.cclnt.Put(srv, string(o.k), o.val)
		// if o.off == 0 {
		// 	_, o.err = fsl.PutFile(fn, 0777, sp.OWRITE, o.b)
		// } else {
		// 	_, o.err = fsl.SetFile(fn, o.b, o.m, o.off)
		// }
	}
	db.DPrintf(db.KVCLERK, "op %v fn %v srv %v s %v err %v", o.kind, fn, srv, s, o.err)
}

func (kc *KvClerk) Get(key string, val proto.Message) error {
	op := &op{GETVAL, val, Tkey(key), 0, sp.OREAD, nil, nil}
	kc.doop(op)
	return op.err
}

func (kc *KvClerk) GetTraced(sctx *tproto.SpanContextConfig, key string, val proto.Message) error {
	return kc.Get(key, val)
}

func (kc *KvClerk) GetReader(k Tkey) (*reader.Reader, error) {
	op := &op{GETRD, nil, k, 0, sp.OREAD, nil, nil}
	kc.doop(op)
	return op.rdr, op.err
}

func (kc *KvClerk) Append(k Tkey, val proto.Message) error {
	op := &op{PUT, val, k, sp.NoOffset, sp.OAPPEND, nil, nil}
	kc.doop(op)
	return op.err
}

func (kc *KvClerk) PutTraced(sctx *tproto.SpanContextConfig, key string, val proto.Message) error {
	return kc.Put(key, val)
}

func (kc *KvClerk) Put(k string, val proto.Message) error {
	op := &op{PUT, val, Tkey(k), 0, sp.OWRITE, nil, nil}
	kc.doop(op)
	return op.err
}

func (kc *KvClerk) DeleteTraced(sctx *tproto.SpanContextConfig, key string) error {
	return kc.Delete(key)
}

func (kc *KvClerk) Delete(k string) error {
	db.DFatalf("Unimplemented")
	return nil
}

func (kc *KvClerk) AppendJson(k string, v proto.Message) error {
	op := &op{PUT, v, Tkey(k), sp.NoOffset, sp.OAPPEND, nil, nil}
	kc.doop(op)
	return op.err
}

// Count the number of keys stored at each group.
func (kc *KvClerk) GetKeyCountsPerGroup(keys []string) map[string]int {
	if err := kc.switchConfig(); err != nil {
		db.DFatalf("Error switching KV config: %v", err)
	}
	cnts := make(map[string]int)
	for _, k := range keys {
		s := key2shard(Tkey(k))
		grp := kc.conf.Shards[s]
		if _, ok := cnts[grp]; !ok {
			cnts[grp] = 0
		}
		cnts[grp]++
	}
	return cnts
}
