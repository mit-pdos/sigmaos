package kv

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfs"
	"ulambda/memfsd"
	np "ulambda/ninep"
)

const (
	KV      = "name/kv"
	SHARDER = "name/kv/sharder"
)

type KvDev struct {
	kv *Kv
}

func (kvdev *KvDev) Write(off np.Toffset, data []byte) (np.Tsize, error) {
	t := string(data)
	log.Printf("KvDev.write %v\n", t)
	if strings.HasPrefix(t, "Reconfigure") {
		kvdev.kv.cond.Signal()
	} else {
		return 0, fmt.Errorf("Write: unknown command %v\n", t)
	}

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
	pid         string
	me          string
	conf        *Config
	inRebalance bool
}

func MakeKv(args []string) (*Kv, error) {
	kv := &Kv{}
	kv.cond = sync.NewCond(&kv.mu)
	if len(args) != 1 {
		return nil, fmt.Errorf("MakeKv: too few arguments %v\n", args)
	}
	log.Printf("Kv: %v\n", args)
	kv.pid = args[0]
	kv.me = KV + "/" + kv.pid

	fs := memfs.MakeRoot(false)
	fsd := memfsd.MakeFsd(false, fs, kv)
	fsl, err := fslib.InitFsMemFsD(kv.me, fs, fsd, &KvDev{kv})
	if err != nil {
		return nil, err
	}
	kv.FsLibSrv = fsl
	db.SetDebug(false)
	kv.Started(kv.pid)
	kv.conf = kv.readConfig()
	return kv, nil
}

func (kv *Kv) join() {
	sh := SHARDER + "/dev"
	log.Printf("Join %v\n", kv.me)
	err := kv.WriteFile(sh, []byte("Join "+kv.me))
	if err != nil {
		log.Printf("WriteFile: %v %v\n", sh, err)
	}
}

// Interposes on memfsd's walk
func (kv *Kv) Walk(src string, names []string) error {
	// log.Printf("%v: Walk %v %v\n", kv.me, src, names)
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if len(names) == 0 { // so that ls in root directory works
		return nil
	}
	if names[0] == "dev" {
		return nil
	}
	if strings.HasPrefix(src, "clerk/") {
		if kv.inRebalance {
			return ErrRetry
		} else {
			p := strings.Split(names[0], "-")
			if p[0] != strconv.Itoa(kv.conf.N) {
				return ErrWrongKv
			}
			shard := key2shard(p[1])
			if kv.conf.Shards[shard] != kv.me {
				return ErrWrongKv
			}
			names[0] = p[1]
			return nil
		}
	}
	return nil
}

func (kv *Kv) Exit() {
	kv.Exiting(kv.pid)
}

func (kv *Kv) readConfig() *Config {
	conf := Config{}
	err := kv.ReadFileJson(KVCONFIG, &conf)
	if err != nil {
		log.Fatalf("ReadFileJson: %v\n", err)
	}
	return &conf
}

func (kv *Kv) copyShard(shard int, kvd string) {
	kv.ProcessDir(kvd, func(st *np.Stat) bool {
		s := key2shard(st.Name)
		if s == shard {
			log.Printf("%v: copy key %v (s %v) from %v\n", kv.me,
				st.Name, s, kvd)
			n := kvd + "/" + st.Name
			b, err := kv.ReadFile(n)
			if err != nil {
				log.Fatalf("%v: Readfile %v err %v\n", kv.me, n, err)
			}
			root := kv.Root()
			rooti := root.RootInode()
			newi, err := rooti.Create(0, root, 0700, st.Name)
			if err != nil {
				log.Fatalf("Create %v err %v\n", st.Name, err)
			}
			_, err = newi.Write(0, b)
			if err != nil {
				log.Fatalf("Write %v err %v\n", st.Name, err)
			}
			err = kv.Remove(n)
			if err != nil {
				log.Fatalf("Remove %v err %v\n", st.Name, err)
			}
		}
		return false
	})

}

// XXX maybe wait until all keys have been copied or don't accept
// another refresh until copier is done (which is not the case because
// the sharder won't initiate another refresh until every KVs is done
// refreshing).
func (kv *Kv) resume() {
	sh := SHARDER + "/dev"
	log.Printf("Resume %v %v\n", kv.me, sh)
	kv.inRebalance = false
	err := kv.WriteFile(sh, []byte("Resume "+kv.me))
	if err != nil {
		log.Printf("WriteFile: %v %v\n", sh, err)
	}
}

// Caller hold lock
func (kv *Kv) reconfigure() {
	kv.inRebalance = true
	conf := kv.readConfig()
	log.Printf("%v: New config: %v\n", kv.me, conf)
	if conf.N > 1 {
		for i, v := range conf.Shards {
			if v == kv.me && kv.conf.Shards[i] != kv.me {
				kv.mu.Unlock()
				kv.copyShard(i, kv.conf.Shards[i])
				kv.mu.Lock()

			}
		}
	}
	kv.conf = conf
	log.Printf("%v: New config installed\n", kv.me)
	kv.resume()
}

func (kv *Kv) Work() {
	kv.mu.Lock()
	kv.join()
	for {
		kv.cond.Wait()
		kv.reconfigure()
	}
}
