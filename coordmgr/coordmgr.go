package coordmgr

import (
	"log"

	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procclnt"
)

//
// Keeps NCOORDs running. If one coordinator crashes, the manager
// starts another one to keep the replication level up.  There are two
// ways of stopping the coordination manager: the caller calls stop or
// the callers calls wait (which returns when the primary coordinator
// returns with an OK status).
//

const (
	NCOORD = 3
)

type CoordMgr struct {
	stop   bool
	coords []*coord
	ch     chan bool
}

type coord struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	pid  string
	bin  string
	args []string
}

type procret struct {
	coord  int
	err    error
	status string
}

func makeCoord(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, bin string, args []string) *coord {
	return &coord{fsl, pclnt, "", bin, args}
}

func (c *coord) spawn() {
	p := proc.MakeProc(c.bin, c.args)
	c.Spawn(p)
	c.WaitStart(p.Pid)
	c.pid = p.Pid
}

func (c *coord) run(i int, start chan bool, done chan procret) {
	//log.Printf("spawn %p coord %v\n", c, c.bin)
	c.spawn()
	//log.Printf("coord %p forked %v\n", c, c.pid)
	start <- true
	status, err := c.WaitExit(c.pid)
	log.Printf("coord %v exited %v err %v\n", c.pid, status, err)
	done <- procret{i, err, status}
}

func StartCoords(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, bin string, args []string) *CoordMgr {
	cm := &CoordMgr{}
	cm.ch = make(chan bool)
	cm.coords = make([]*coord, NCOORD)
	for i := 0; i < NCOORD; i++ {
		cm.coords[i] = makeCoord(fsl, pclnt, bin, args)
	}
	done := make(chan procret)
	for i, c := range cm.coords {
		start := make(chan bool)
		go c.run(i, start, done)
		<-start
	}
	go cm.manager(done)
	return cm
}

func (cm *CoordMgr) manager(done chan procret) {
	n := NCOORD
	for n > 0 {
		st := <-done
		if cm.stop {
			n--
		} else if st.err == nil && st.status == "OK" { // done?
			cm.stop = true
			n--
		} else { // restart coord i
			start := make(chan bool)
			go cm.coords[st.coord].run(st.coord, start, done)
			<-start
		}
	}
	cm.ch <- true
}

func (cm *CoordMgr) Wait() {
	<-cm.ch
}

func (cm *CoordMgr) StopCoords() error {
	// coordinators may not run in order of coords, and blocked
	// waiting for Wlease, while the primary keeps running,
	// because it is later in the list.
	cm.stop = true
	var err error
	for _, c := range cm.coords {
		go func(c *coord) {
			// log.Printf("evict %v\n", c.pid)
			r := c.Evict(c.pid)
			if r != nil {
				err = r
			}
		}(c)
	}
	// log.Printf("wait for coordinators\n")
	<-cm.ch
	log.Printf("done coordinators\n")
	return err
}
