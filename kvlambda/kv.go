package kvlambda

import (
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfsd"
	np "ulambda/ninep"
	"ulambda/npsrv"
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
	if strings.HasPrefix(t, "Split") {
		keys := strings.TrimLeft(t, "Split ")
		kvdev.kv.split(keys)
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
	mu     sync.Mutex
	cond   *sync.Cond
	clnt   *fslib.FsLib
	memfsd *memfsd.Fsd
	srv    *npsrv.NpServer
	pid    string
	krange []string
	parent []string
}

func MakeKv(args []string) (*Kv, error) {
	kv := &Kv{}
	kv.cond = sync.NewCond(&kv.mu)
	kv.clnt = fslib.MakeFsLib(false)
	kv.memfsd = memfsd.MakeFsd(false)
	kv.srv = npsrv.MakeNpServer(kv.memfsd, ":0", false)
	if len(args) != 3 {
		return nil, fmt.Errorf("MakeKv: too few arguments %v\n", args)
	}
	log.Printf("Kv: %v\n", args)
	kv.pid = args[0]
	kv.krange = strings.Split(args[1], "-")
	kv.parent = strings.Split(args[2], "-")

	if fd, err := kv.clnt.Attach(":1111", ""); err == nil {
		err := kv.clnt.Mount(fd, "name")
		if err != nil {
			log.Fatal("Mount error: ", err)
		}
		err = kv.clnt.Remove(KV + "/" + args[1])
		if err != nil {
			db.DPrintf("Remove failed %v\n", err)
		}

		fs := kv.memfsd.Root()
		_, err = fs.MkNod(fs.RootInode(), DEV, &KvDev{kv})
		if err != nil {
			log.Fatal("Create error: ", err)
		}

		name := kv.srv.MyAddr()
		err = kv.clnt.Symlink(name+":pubkey:kv"+args[1], KV+"/"+args[1], 0777)
		if err != nil {
			log.Fatal("Symlink error: ", err)
		}
		db.SetDebug(false)
		kv.clnt.Started(kv.pid)
		kv.mvKeys()
		return kv, nil
	} else {
		log.Fatal("Attach error: ", err)
		return nil, err
	}
}

// XXX move keys
func (kv *Kv) mvKeys() {
	log.Printf("parent %v\n", kv.parent)
	if len(kv.parent) < 2 {
		return
	}
	dst := kv.parent[0] + "-" + kv.krange[0]
	src := kv.parent[0] + "-" + kv.parent[1]
	log.Printf("rename src %v to %v\n", src, dst)
	err := kv.clnt.Rename(KV+"/"+src, KV+"/"+dst)
	if err != nil {
		log.Fatalf("Rename %v to %v failed %v\n", src, dst, err)
	}
	pdev := KV + "/" + dst + "/" + DEV
	mykeys := kv.krange[0] + "-" + kv.krange[1]
	log.Printf("mv keys %v parent %v\n", mykeys, pdev)
	err = kv.clnt.WriteFile(pdev, []byte("Split "+mykeys))
	if err != nil {
		log.Printf("WriteFile: %v %v\n", pdev, err)
	}
	kv.parent[1] = kv.krange[0]
}

func (kv *Kv) split(krange string) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	r := strings.Split(krange, "-")
	kv.krange[1] = r[0]
	log.Printf("split %v -> my range %v %v\n", r, kv.krange[0], kv.krange[1])
}

func (kv *Kv) Exit() {
	kv.clnt.Exiting(kv.pid)
}

func (kv *Kv) Work() {
	kv.mu.Lock()
	for {
		kv.mu.Unlock()
		time.Sleep(time.Duration(10000) * time.Millisecond)
		kv.mu.Lock()
		// pretend cpu is busy
		pid := strconv.Itoa(rand.Intn(100000))
		log.Printf("my range %v %v\n", kv.krange[0], kv.krange[1])
		n1, err := strconv.Atoi(kv.krange[0])
		n2, err := strconv.Atoi(kv.krange[1])
		if err != nil {
			log.Fatalf("strconv failed %v\n", err)
		}
		s := strconv.Itoa((n2-n1)/2 + n1)
		a := fslib.Attr{pid, "./bin/kvd",
			[]string{s + "-" + kv.krange[1],
				kv.krange[0] + "-" + kv.krange[1]},
			[]fslib.PDep{fslib.PDep{kv.pid, pid}},
			nil}
		err = kv.clnt.Spawn(&a)
		if err != nil {
			log.Fatalf("Spawn failed %v\n", err)
		}

	}
	kv.cond.Wait()
}
