package kv

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/big"
	"strconv"
	"strings"
	"time"

	db "sigmaos/debug"
	"sigmaos/fenceclnt"
	"sigmaos/fslib"
	"sigmaos/group"
	np "sigmaos/sigmap"
	"sigmaos/procclnt"
	"sigmaos/reader"
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

func MkKey(k uint64) Tkey {
	return Tkey(strconv.FormatUint(k, 16))
}

type Tshard int

func (s Tshard) String() string {
	return fmt.Sprintf("%03d", s)
}

func key2shard(key Tkey) Tshard {
	h := fnv.New32a()
	h.Write([]byte(key))
	shard := Tshard(h.Sum32() % NSHARD)
	return shard
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
	*procclnt.ProcClnt
	fclnt *fenceclnt.FenceClnt
	conf  *Config
	job   string
}

func MakeClerkFsl(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, job string) (*KvClerk, error) {
	return makeClerk(fsl, pclnt, job)
}

func MakeClerk(name, job string, namedAddr []string) (*KvClerk, error) {
	fsl := fslib.MakeFsLibAddr(name, namedAddr)
	pclnt := procclnt.MakeProcClnt(fsl)
	return makeClerk(fsl, pclnt, job)
}

func makeClerk(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, job string) (*KvClerk, error) {
	kc := &KvClerk{}
	kc.FsLib = fsl
	kc.ProcClnt = pclnt
	kc.conf = &Config{}
	kc.job = job
	kc.fclnt = fenceclnt.MakeLeaderFenceClnt(kc.FsLib, KVBalancer(kc.job))
	if err := kc.switchConfig(); err != nil {
		return nil, err
	}
	return kc, nil
}

// Detach servers not in kvs
func (kc *KvClerk) DetachKVs(kvs *KvSet) {
	mnts := kc.Mounts()
	for _, mnt := range mnts {
		if strings.HasPrefix(mnt, group.JobDir(JobDir(kc.job))) {
			kvd := strings.TrimPrefix(mnt, group.JobDir(JobDir(kc.job))+"/")
			if !kvs.present(kvd) {
				db.DPrintf("KVCLERK0", "Detach kv %v", kvd)
				kc.Detach(group.GrpPath(JobDir(kc.job), kvd))
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
			db.DPrintf("KVCLERK_ERR", "GetFileJsonWatch %v err %v", KVConfig(kc.job), err)
			return err
		}
		db.DPrintf("KVCLERK", "Conf %v", kc.conf)
		kvset := MakeKvs(kc.conf.Shards)
		dirs := paths(kc.job, kvset)
		if err := kc.fclnt.FenceAtEpoch(kc.conf.Epoch, dirs); err != nil {
			if fcall.IsErrVersion(err) || fcall.IsErrStale(err) {
				db.DPrintf("KVCLERK_ERR", "version mismatch; retry")
				time.Sleep(WAITMS * time.Millisecond)
				continue
			}
			db.DPrintf("KVCLERK_ERR", "FenceAtEpoch %v failed %v", dirs, err)
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
	if fcall.IsErrNotfound(err) && strings.HasPrefix(fcall.ErrPath(err), "shard") {
		// Shard dir hasn't been created yet (config 0) or hasn't moved
		// yet, so wait a bit, and retry.  XXX make sleep time
		// dynamic?
		db.DPrintf("KVCLERK_ERR", "Wait for shard %v", fcall.ErrPath(err))
		time.Sleep(WAITMS * time.Millisecond)
		return nil
	}
	if fcall.IsErrStale(err) {
		db.DPrintf("KVCLERK_ERR", "fixRetry %v", err)
		return kc.switchConfig()
	}
	return err
}

// Do an operation. If an error, try to fix the error (e.g., rereading
// config), and on success, retry.
func (kc *KvClerk) doop(o *op) {
	s := key2shard(o.k)
	for {
		db.DPrintf("KVCLERK", "o %v conf %v", o.kind, kc.conf)
		fn := keyPath(kc.job, kc.conf.Shards[s], s, o.k)
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
	k    Tkey
	off  np.Toffset
	m    np.Tmode
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
		_, o.err = fsl.SetFile(fn, o.b, o.m, o.off)
	}
	db.DPrintf("KVCLERK", "op %v fn %v err %v", o.kind, fn, o.err)
}

func (kc *KvClerk) Get(k Tkey, off np.Toffset) ([]byte, error) {
	op := &op{GETVAL, []byte{}, k, off, np.OREAD, nil, nil}
	kc.doop(op)
	return op.b, op.err
}

func (kc *KvClerk) GetReader(k Tkey) (*reader.Reader, error) {
	op := &op{GETRD, []byte{}, k, 0, np.OREAD, nil, nil}
	kc.doop(op)
	return op.rdr, op.err
}

func (kc *KvClerk) Set(k Tkey, b []byte, off np.Toffset) error {
	op := &op{SET, b, k, off, np.OWRITE, nil, nil}
	kc.doop(op)
	return op.err
}

func (kc *KvClerk) Append(k Tkey, b []byte) error {
	op := &op{SET, b, k, np.NoOffset, np.OAPPEND, nil, nil}
	kc.doop(op)
	return op.err
}

func (kc *KvClerk) Put(k Tkey, b []byte) error {
	op := &op{PUT, b, k, 0, np.OWRITE, nil, nil}
	kc.doop(op)
	return op.err
}

func (kc *KvClerk) AppendJson(k Tkey, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	op := &op{SET, b, k, np.NoOffset, np.OAPPEND, nil, nil}
	kc.doop(op)
	return op.err
}

// Count the number of keys stored at each group.
func (kc *KvClerk) GetKeyCountsPerGroup(keys []Tkey) map[string]int {
	if err := kc.switchConfig(); err != nil {
		db.DFatalf("Error switching KV config: %v", err)
	}
	cnts := make(map[string]int)
	for _, k := range keys {
		s := key2shard(k)
		grp := kc.conf.Shards[s]
		if _, ok := cnts[grp]; !ok {
			cnts[grp] = 0
		}
		cnts[grp]++
	}
	return cnts
}
