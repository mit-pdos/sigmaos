package ulambd

import (
	"encoding/json"
	//"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"
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
	MAXLOAD = 1
)

type LambdDev struct {
	ld *Lambd
}

func (ldev *LambdDev) Write(off np.Toffset, data []byte) (np.Tsize, error) {
	ldev.ld.mu.Lock()
	defer ldev.ld.mu.Unlock()

	t := string(data)
	log.Printf("LambdDev.write %v\n", t)
	if strings.HasPrefix(t, "Started") {
		pid := strings.TrimLeft(t, "Started ")
		ldev.ld.started(pid)
	} else if strings.HasPrefix(t, "Start") {
		ldev.ld.getLambdas()
		ldev.ld.cond.Signal()
	} else {
		return 0, fmt.Errorf("Write: unknown command %v\n", t)
	}
	return np.Tsize(len(data)), nil
}

func (ldev *LambdDev) Read(off np.Toffset, n np.Tsize) ([]byte, error) {
	if off == 0 {
		s := ldev.ld.String()
		return []byte(s), nil
	}
	return nil, nil
}

func (ldev *LambdDev) Len() np.Tlength {
	return np.Tlength(len(ldev.ld.String()))
}

type Lambd struct {
	mu     sync.Mutex
	cond   *sync.Cond
	clnt   *fslib.FsLib
	memfsd *memfsd.Fsd
	srv    *npsrv.NpServer
	load   int
	ls     map[string]*Lambda
}

func MakeLambd() *Lambd {
	ld := &Lambd{}
	ld.cond = sync.NewCond(&ld.mu)
	ld.clnt = fslib.MakeFsLib(false)
	ld.memfsd = memfsd.MakeFsd(false)
	ld.srv = npsrv.MakeNpServer(ld.memfsd, ":0", false)
	ld.load = 0
	ld.ls = make(map[string]*Lambda)

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

	err = ld.clnt.Mkdir("name/ulambd/pids", 0777)
	if err != nil {
		log.Fatal("Mkdir error: ", err)
	}

	return ld
}

func (ld *Lambd) String() string {
	s := ""
	for _, l := range ld.ls {
		s += fmt.Sprintf("%v\n", l)

	}
	return s
}

func (ld *Lambd) ReadLambda(pid string) (*Lambda, error) {
	l := &Lambda{}
	l.path = LDIR + pid
	l.pid = pid
	l.ld = ld
	dirents, err := ld.clnt.ReadDir(l.path)
	if err != nil {
		return nil, err
	}
	l.afterStart = make(map[string]bool)
	l.afterExit = make(map[string]bool)
	for _, d := range dirents {
		if d.Name == "attr" {
			b, err := ld.clnt.ReadFile(l.path + "/attr")
			if err != nil {
				return nil, err
			}
			var attr Attr
			err = json.Unmarshal(b, &attr)
			if err != nil {
				log.Fatal("Unmarshal error ", err)
			}
			l.program = attr.Program
			l.args = attr.Args
			for _, p := range attr.AfterStart {
				l.afterStart[p] = true
			}
			for _, p := range attr.AfterExit {
				l.afterExit[p] = true
			}
		} else {
			l.status = d.Name
		}
	}
	return l, nil
}

func (ld *Lambd) getLambdas() {
	ld.clnt.ProcessDir(LDIR, func(st *np.Stat) bool {
		l, err := ld.ReadLambda(st.Name)
		if err != nil {
			log.Fatalf("ReadLambda st.Name %v error %v ", st.Name, err)
		}
		_, ok := ld.ls[st.Name]
		if !ok {
			ld.ls[st.Name] = l
		}
		return false
	})
}

func (ld *Lambd) started(path string) {
	log.Printf("started %v\n", path)
	pid := filepath.Base(path)
	for _, l := range ld.ls {
		if l.afterStart[pid] {
			delete(l.afterStart, pid)
			if l.runnable() {
				l.run() // XXX run consumer, irrespective of load
				ld.load += 1
			}
		}
	}
}

func (ld *Lambd) runLambda(l *Lambda) {
	if ld.load < MAXLOAD {
		err := l.run()
		if err != nil {
			log.Printf("Run: Error %v\n", err)
		} else {
			ld.load += 1
		}
	}
}

func (ld *Lambd) findRunnable() *Lambda {
	// first look for a waiting one whose start predecessor has started
	// and has no exit dependencies.
	for _, l := range ld.ls {
		if l.status == "Waiting" && l.runnable() {
			return l
		}
	}
	// look for a runnable one
	for _, l := range ld.ls {
		if l.status == "Runnable" {
			return l
		}
	}
	return nil
}

func (ld *Lambd) exit(l *Lambda) error {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	log.Printf("exit %v\n", l.path)
	err := ld.clnt.Remove(l.path)
	if err != nil {
		log.Fatalf("Remove %v error %v\n", l.path, err)
	}
	l, ok := ld.ls[l.pid]
	if !ok {
		log.Fatalf("exit: unknown %v\n", l.pid)
	}
	delete(ld.ls, l.pid)
	ld.load -= 1
	for _, m := range ld.ls {
		if m.afterExit[l.pid] {
			delete(m.afterExit, l.pid)
		}
	}
	ld.cond.Signal()
	return nil
}

func (ld *Lambd) Scheduler() {
	ld.mu.Lock()
	ld.getLambdas()
	for {
		log.Printf("ls %v\n", ld)
		l := ld.findRunnable()
		if l != nil {
			ld.runLambda(l)
		}
		if l == nil || ld.load >= MAXLOAD {
			log.Printf("Nothing to do or busy %v", ld.load)
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
