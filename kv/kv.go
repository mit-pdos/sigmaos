package kv

import (
	"fmt"
	"log"
	"sync"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfs"
	"ulambda/memfsd"
	np "ulambda/ninep"
)

const (
	KV    = "name/kv"
	DEV   = "dev"
	KVDEV = KV + "/" + DEV
)

type KvDev struct {
	kv *Kv
}

func (kvdev *KvDev) Write(off np.Toffset, data []byte) (np.Tsize, error) {
	t := string(data)
	log.Printf("KvDev.write %v\n", t)
	return np.Tsize(len(data)), nil
}

func (kvdev *KvDev) Read(off np.Toffset, n np.Tsize) ([]byte, error) {
	//	if off == 0 {
	//	s := kvdev.sd.ps()
	//return []byte(s), nil
	//}
	return nil, nil
}

func (kvdev *KvDev) Len() np.Tlength {
	return 0
}

type Kv struct {
	mu   sync.Mutex
	cond *sync.Cond
	*fslib.FsLibSrv
	pid  string
	me   string
	conf Config
}

func MakeKv(args []string) (*Kv, error) {
	kv := &Kv{}
	kv.cond = sync.NewCond(&kv.mu)
	if len(args) != 2 {
		return nil, fmt.Errorf("MakeKv: too few arguments %v\n", args)
	}
	log.Printf("Kv: %v\n", args)
	kv.pid = args[0]
	kv.me = KV + "/" + args[1]

	fs := memfs.MakeRoot(false)
	fsd := memfsd.MakeFsd(false, fs, kv, kv)
	fsl, err := fslib.InitFsMemFsD(kv.me, fs, fsd, &KvDev{kv})
	if err != nil {
		return nil, err
	}
	kv.FsLibSrv = fsl
	db.SetDebug(false)
	kv.Started(kv.pid)
	kv.register()
	return kv, nil
}

// XXX move keys
func (kv *Kv) register() {
	sh := KV + "/sharder/" + DEV
	log.Printf("register %v %v\n", kv.me, sh)
	err := kv.WriteFile(sh, []byte("Add "+kv.me))
	if err != nil {
		log.Printf("WriteFile: %v %v\n", sh, err)
	}
	err = kv.ReadFileJson(KVCONFIG, &kv.conf)
	if err != nil {
		log.Printf("ReadFileJson: %v\n", err)
	}

}

func (kv *Kv) Open(path string, mode np.Tmode) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	log.Printf("Open %v\n", path)
	shard := key2shard(path)
	if kv.conf.Shards[shard] != kv.me {
		return ErrWrongKv
	}
	return nil
}

func (kv *Kv) Create(path string, perm np.Tperm, mode np.Tmode) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	log.Printf("Create %v\n", path)
	shard := key2shard(path)
	if kv.conf.Shards[shard] != kv.me {
		return ErrWrongKv
	}
	return nil
}

func (kv *Kv) Exit() {
	kv.Exiting(kv.pid)
}

func (kv *Kv) Work() {
	kv.mu.Lock()
	for {
		kv.cond.Wait()
	}
}
