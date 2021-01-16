package ulambd

import (
	"encoding/json"
	"log"
	"time"

	"ulambda/fslib"
	np "ulambda/ninep"
)

const LDIR = "name/ulambda/"

type Lambd struct {
	clnt *fslib.FsLib
	load int
}

func MakeLambd() *Lambd {
	ld := &Lambd{}
	ld.clnt = fslib.MakeFsLib(false)
	ld.load = 0
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
		log.Fatalf("ReadLambda st.Name %v error %v ", err)
		return false
	}
	log.Printf("Sched %v: %v\n", ld.load, l)
	for {
		if l.status == "Runnable" {
			if ld.load <= 3 {
				err = l.run()
				if err != nil {
					log.Printf("Run: Error %v\n", err)
				}
				ld.load += 1
				return true
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
	for {
		empty, err := ld.clnt.ProcessDir(LDIR, ld.RunLambda)
		if err != nil {
			log.Fatalf("Run: error %v\n", err)
		}
		if empty {
			log.Print("Run done")
			return
		}
		time.Sleep(time.Duration(1000) * time.Millisecond)
	}
}
