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

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfsd"
	np "ulambda/ninep"
	"ulambda/npsrv"
)

// XXX monitor, boost

const (
	LDIR    = "name/ulambd/pids/"
	MAXLOAD = 1 // XXX bogus controls parallelism
)

type LambdDev struct {
	ld *Lambd
}

func (ldev *LambdDev) Write(off np.Toffset, data []byte) (np.Tsize, error) {
	ldev.ld.mu.Lock()
	defer ldev.ld.mu.Unlock()

	t := string(data)
	db.DPrintf("LambdDev.write %v\n", t)
	if strings.HasPrefix(t, "Spawn") {
		l := strings.TrimLeft(t, "Spawn ")
		ldev.ld.spawn(l)
		ldev.ld.cond.Signal()
	} else if strings.HasPrefix(t, "Started") {
		pid := strings.TrimLeft(t, "Started ")
		ldev.ld.started(pid)
		// ldev.ld.cond.Signal()
	} else {
		return 0, fmt.Errorf("Write: unknown command %v\n", t)
	}
	return np.Tsize(len(data)), nil
}

func (ldev *LambdDev) Read(off np.Toffset, n np.Tsize) ([]byte, error) {
	ldev.ld.mu.Lock()
	defer ldev.ld.mu.Unlock()

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
	load   int // XXX bogus
	ls     map[string]*Lambda
}

func MakeLambd(debug bool) *Lambd {
	ld := &Lambd{}
	ld.cond = sync.NewCond(&ld.mu)
	ld.clnt = fslib.MakeFsLib(false)
	ld.memfsd = memfsd.MakeFsd(false)
	ld.srv = npsrv.MakeNpServer(ld.memfsd, ":0", false)
	ld.load = 0
	ld.ls = make(map[string]*Lambda)

	err := ld.clnt.Remove("name/ulambd")
	if err != nil {
		db.DPrintf("name/ulambd didn't exist")
	}
	name := ld.srv.MyAddr()
	err = ld.clnt.Symlink(name+":pubkey:console", "name/ulambd", 0777)
	if err != nil {
		log.Fatal("Symlink error: ", err)
	}

	fs := ld.memfsd.Root()
	_, err = fs.MkNod(fs.RootInode(), "ulambd", &LambdDev{ld})
	if err != nil {
		log.Fatal("Create error: ", err)
	}

	err = ld.clnt.Mkdir("name/ulambd/pids", 0777)
	if err != nil {
		log.Fatal("Mkdir error: ", err)
	}
	db.SetDebug(debug)
	return ld
}

func (ld *Lambd) String() string {
	s := ""
	for _, l := range ld.ls {
		s += fmt.Sprintf("%v\n", l)

	}
	return s
}

func (ld *Lambd) spawn(ls string) {
	l := &Lambda{}
	splits := strings.SplitN(ls, " ", 2)
	l.pid = splits[0]
	l.ld = ld
	l.afterStart = make(map[string]bool)
	l.afterExit = make(map[string]bool)
	var attr Attr
	err := json.Unmarshal([]byte(splits[1]), &attr)
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
	_, ok := ld.ls[l.pid]
	if !ok {
		ld.ls[l.pid] = l
	} else {
		log.Fatalf("Spawn %v already exists\n", l.pid)
	}
	if l.runnable() {
		l.status = "Runnable"
	} else {
		l.status = "Waiting"
	}
	db.DPrintf("Spawn %v\n", l)
}

func (ld *Lambd) started(path string) {
	db.DPrintf("started %v\n", path)
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

	db.DPrintf("exit %v\n", l.pid)
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
	for {
		l := ld.findRunnable()
		if l != nil {
			ld.runLambda(l)
		}
		if l == nil || ld.load >= MAXLOAD {
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
