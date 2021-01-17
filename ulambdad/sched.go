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

const (
	LDIR    = "name/ulambd/pids/"
	MAXLOAD = 3
)

type LambdDev struct {
	ld *Lambd
}

func (ldev *LambdDev) Write(off np.Toffset, data []byte) (np.Tsize, error) {
	log.Printf("LambdDev.write %v\n", data)
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

	err := ld.clnt.Remove("name/ulambd")
	if err != nil {
		log.Print("name/ulambd didn't exist")
	}
	name := ld.srv.MyAddr()
	err = ld.clnt.Symlink(name+":pubkey:console", "name/ulambd", 0777)
	if err != nil {
		log.Fatal("Symlink error: ", err)
	}

	// XXX use local interface for MkNod
	fs := ld.memfsd.Root()
	_, err = fs.MkNod(fs.RootInode(), "ulambd", &LambdDev{ld})
	if err != nil {
		log.Fatal("Create error: ", err)
	}

	// XXX this sets up TCP connection with the server in this
	// process; it would be nice to be more efficient.  We
	// shouldn't peak inside memfsd's data structures directly,
	// perhaps it is not memfs, but persistent, or replicated etc.
	// FsClnt could call memfsd's 9p conn directly (i.e., have two
	// implementations for 9pclnt), but fsclnt would need to know
	// about memfsd.  We would stil pay then for marshalling of
	// directory entries when reading a directory.  Perhaps we
	// should pay the overhead of the TCP connection, and not have
	// a local memfsd at all (e.g., just use named).
	// Alternatively, ulambd is itself its own 9p service, not
	// using memfsd, but perhaps just memfs, then we can
	// specialize and make it performant in any way we want.
	err = ld.clnt.Mkdir("name/ulambd/pids", 0777)
	if err != nil {
		log.Fatal("Mkdir error: ", err)
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

func (ld *Lambd) runLambda(l *Lambda) {
	if ld.load <= MAXLOAD {
		err := l.run()
		if err != nil {
			log.Printf("Run: Error %v\n", err)
		} else {
			ld.load += 1
		}
	}
}

// Process a lambda, skipping Waiting and Running ones
func (ld *Lambd) processLambda(st *np.Stat) bool {
	l, err := ld.ReadLambda(LDIR + st.Name)
	if err != nil {
		log.Fatalf("ReadLambda st.Name %v error %v ", st.Name, err)
	}
	log.Printf("Sched %v: %v\n", ld.load, l)
	if l.status == "Runnable" {
		ld.runLambda(l)
	} else if l.status == "Waiting" {
		if l.isRunnable(ld.Getpids()) {
			ld.runLambda(l)
		} else {
			return false
		}
	} else if l.status == "Running" {
		return false
	} else if l.status == "Exit" {
		ld.load -= 1
		l.exit()
	} else {
		log.Fatalf("Unknown status %v\n", l.status)
	}
	return true
}

func (ld *Lambd) Scheduler() {
	ld.mu.Lock()
	for { /// l.load
		stopped, err := ld.clnt.ProcessDir(LDIR, ld.processLambda)
		if err != nil {
			log.Fatalf("ProcessDir: error %v\n", err)
		}
		if !stopped || ld.load >= MAXLOAD {
			log.Print("Nothing to do")
			ld.cond.Wait()
		}
		// time.Sleep(time.Duration(1) * time.Millisecond)
	}
}

// log.Printf("in progress: %v\n", st.Name)
// timeout := int64(st.Mtime) + 5
// if timeout < time.Now().Unix() {
// 	log.Print("REDO ", st.Name)
// 	err = md.clnt.Rename("name/mr/started/"+st.Name,
// 		"name/mr/todo/"+st.Name)
// }
