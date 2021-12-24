package coordmgr

import (
	"log"

	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procclnt"
)

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

func makeCoord(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, bin string, args []string) *coord {
	return &coord{fsl, pclnt, "", bin, args}
}

func (c *coord) spawn() {
	p := proc.MakeProc(c.bin, c.args)
	c.Spawn(p)
	c.WaitStart(p.Pid)
	c.pid = p.Pid
}

func (c *coord) run(i int, start chan bool, done chan int) {
	log.Printf("spawn %p coord %v\n", c, c.bin)
	c.spawn()
	log.Printf("coord %p forked %v\n", c, c.pid)
	start <- true
	status, err := c.WaitExit(c.pid)
	log.Printf("coord %v exited %v err %v\n", c.pid, status, err)
	done <- i
}

func StartCoords(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, bin string, args []string) *CoordMgr {
	cm := &CoordMgr{}
	cm.ch = make(chan bool)
	cm.coords = make([]*coord, NCOORD)
	for i := 0; i < NCOORD; i++ {
		cm.coords[i] = makeCoord(fsl, pclnt, bin, args)
	}
	done := make(chan int)
	for i, c := range cm.coords {
		start := make(chan bool)
		go c.run(i, start, done)
		<-start
	}
	go cm.manager(done)
	return cm
}

func (cm *CoordMgr) manager(done chan int) {
	n := NCOORD
	for n > 0 {
		ci := <-done
		if cm.stop {
			n--
		} else { // restart i
			start := make(chan bool)
			go cm.coords[ci].run(ci, start, done)
			<-start
		}
	}
	cm.ch <- true
}

func (cm *CoordMgr) StopCoords() error {
	// coordinators may not run in order of coords, and blocked
	// waiting for Wlease, while the primary keeps running,
	// because it is later in the list.
	cm.stop = true
	var err error
	for _, c := range cm.coords {
		go func(c *coord) {
			log.Printf("evict %v\n", c.pid)
			r := c.Evict(c.pid)
			if r != nil {
				err = r
			}
		}(c)
	}
	log.Printf("wait for coordinators\n")
	<-cm.ch
	log.Printf("done coordinators\n")
	return err
}
