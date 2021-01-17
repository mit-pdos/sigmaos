package ulambd

import (
	"encoding/json"
	"errors"
	"log"
	"sync"
	// "time"

	"ulambda/fslib"
	"ulambda/memfsd"
	np "ulambda/ninep"
	"ulambda/npsrv"
)

// XXX monitor, boost
// device file to kick it into action

const (
	LDIR    = "name/ulambd/pids/" // XXX use local name, no client
	MAXLOAD = 7
)

type LambdDev struct {
	ld *Lambd
}

func (ldev *LambdDev) Write(off np.Toffset, data []byte) (np.Tsize, error) {
	log.Printf("write %v\n", data)
	ldev.ld.cond.Signal()
	return np.Tsize(len(data)), nil
}

func (ldev *LambdDev) Read(off np.Toffset, n np.Tsize) ([]byte, error) {
	return nil, errors.New("Unsupported")
}

func (ldev *LambdDev) Len() np.Tlength { return 0 }

type Lambd struct {
	mu     sync.Mutex
	cond   *sync.Cond
	clnt   *fslib.FsLib
	memfsd *memfsd.Fsd
	srv    *npsrv.NpServer
	load   int
}

func MakeLambd() *Lambd {
	ld := &Lambd{}
	ld.cond = sync.NewCond(&ld.mu)
	ld.clnt = fslib.MakeFsLib(false)
	ld.memfsd = memfsd.MakeFsd(false)
	ld.srv = npsrv.MakeNpServer(ld.memfsd, ":0", false)
	ld.load = 0
	name := ld.srv.MyAddr()
	err := ld.clnt.Symlink(name+":pubkey:console", "name/ulambd", 0777)
	if err != nil {
		log.Fatal("Symlink error: ", err)
	}
	fs := ld.memfsd.Root()
	_, err = fs.MkNod(fs.RootInode(), "ulambd", &LambdDev{ld})
	if err != nil {
		log.Fatal("Create error: ", err)
	}
	rooti := fs.RootInode()
	_, err = rooti.Create(0, fs, np.DMDIR|07000, "pids")
	if err != nil {
		log.Fatal("Create error: ", err)
	}
	return ld
}

func (ld *Lambd) ReadLambda(dir string) (*Lambda, error) {
	dirents, err := ld.clnt.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	l := &Lambda{}
	l.pid = dir
	l.clnt = ld.clnt
	for _, d := range dirents {
		if d.Name == "attr" {
			b, err := ld.clnt.ReadFile(dir + "/attr")
			if err != nil {
				return nil, err
			}
			err = json.Unmarshal(b, &l.attr)
			if err != nil {
				log.Fatal("Unmarshal error ", err)
			}
		} else {
			l.status = d.Name
		}
	}
	return l, nil
}

func (ld *Lambd) Getpids() map[string]bool {
	pids := map[string]bool{}
	ld.clnt.ProcessDir(LDIR, func(st *np.Stat) bool {
		pids[st.Name] = true
		return false
	})
	return pids
}

// XXX process every lambda
func (ld *Lambd) RunLambda(st *np.Stat) bool {
	l, err := ld.ReadLambda(LDIR + st.Name)
	if err != nil {
		log.Fatalf("ReadLambda st.Name %v error %v ", st.Name, err)
		return false
	}
	log.Printf("Sched %v: %v\n", ld.load, l)
	for {
		if l.status == "Runnable" {
			if ld.load <= MAXLOAD {
				err = l.run()
				if err != nil {
					log.Printf("Run: Error %v\n", err)
				}
				ld.load += 1
				return false
			}
			return false
		} else if l.status == "Waiting" {
			if !l.isRunnable(ld.Getpids()) {
				return false
			}
			// run l
		} else if l.status == "Running" {
			// XXX monitor progress?
			return false
		} else if l.status == "Exit" {
			ld.load -= 1
			l.exit()
			return false
		} else {
			log.Fatalf("Unknown status %v\n", l.status)
		}
	}
	return true
}

func (ld *Lambd) Run() {
	ld.mu.Lock()
	for {
		empty, err := ld.clnt.ProcessDir(LDIR, ld.RunLambda)
		if err != nil {
			log.Fatalf("Run: error %v\n", err)
		}
		if empty {
			log.Print("Nothing to do")
		}
		// time.Sleep(time.Duration(1) * time.Millisecond)
		ld.cond.Wait()
	}
}
