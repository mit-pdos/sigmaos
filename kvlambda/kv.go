package kvlambda

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	db "ulambda/debug"
	"ulambda/fslib"
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
	fls  *fslib.FsLibSrv
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
	fls, err := fslib.InitFs(kv.me, &KvDev{kv})
	if err != nil {
		return nil, err
	}
	kv.fls = fls
	db.SetDebug(false)
	kv.fls.Started(kv.pid)
	kv.register()
	return kv, nil
}

// XXX move keys
func (kv *Kv) register() {
	pdev := KV + "/sharder/" + DEV
	log.Printf("register %v %v\n", kv.me, pdev)
	err := kv.fls.WriteFile(pdev, []byte("Add "+kv.me))
	if err != nil {
		log.Printf("WriteFile: %v %v\n", pdev, err)
	}
	kv.readConfig()
}

func (kv *Kv) readConfig() {
	b, err := kv.fls.ReadFile(KV + "/sharder/config")
	if err != nil {
		log.Fatal("Read config error ", err)
	}
	err = json.Unmarshal(b, &kv.conf)
	if err != nil {
		log.Fatal("Unmarshal error ", err)
	}
	log.Printf("conf = %v\n", kv.conf)
}

func (kv *Kv) Exit() {
	kv.fls.Exiting(kv.pid)
}

func (kv *Kv) Work() {
	kv.mu.Lock()
	for {
		kv.cond.Wait()
	}
}
