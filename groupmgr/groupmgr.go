package groupmgr

import (
	"log"
	"strconv"

	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procclnt"
)

//
// Keeps n members running. If one member crashes, the manager starts
// another one to keep the replication level up.  There are two ways
// of stopping the group manager: the caller calls stop or the callers
// calls wait (which returns when the primary member returns with an
// OK status).
//

type GroupMgr struct {
	stop    bool
	members []*member
	ch      chan bool
}

type member struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	pid   string
	bin   string
	args  []string
	crash int
}

type procret struct {
	member int
	err    error
	status *proc.Status
}

func makeMember(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, bin string, args []string, crash int) *member {
	return &member{fsl, pclnt, "", bin, args, crash}
}

func (m *member) spawn() {
	p := proc.MakeProc(m.bin, m.args)
	p.AppendEnv("SIGMACRASH", strconv.Itoa(m.crash))
	m.Spawn(p)
	m.WaitStart(p.Pid)
	m.pid = p.Pid
}

func (m *member) run(i int, start chan bool, done chan procret) {
	//log.Printf("spawn %p member %v\n", c, m.bin)
	m.spawn()
	//log.Printf("member %p forked %v\n", c, m.pid)
	start <- true
	status, err := m.WaitExit(m.pid)
	//log.Printf("member %v exited %v err %v\n", m.pid, status, err)
	done <- procret{i, err, status}
}

// ncrash = number of group members which may crash.
func Start(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, n int, bin string, args []string, ncrash, crash int) *GroupMgr {
	gm := &GroupMgr{}
	gm.ch = make(chan bool)
	gm.members = make([]*member, n)
	for i := 0; i < n; i++ {
		crashMember := crash
		if i+1 > ncrash {
			crashMember = 0
		}
		gm.members[i] = makeMember(fsl, pclnt, bin, args, crashMember)
	}
	done := make(chan procret)
	for i, m := range gm.members {
		start := make(chan bool)
		go m.run(i, start, done)
		<-start
	}
	go gm.manager(done, n)
	return gm
}

func (gm *GroupMgr) manager(done chan procret, n int) {
	for n > 0 {
		st := <-done
		if gm.stop {
			n--
		} else if st.err == nil && st.status.IsStatusOK() { // done?
			gm.stop = true
			n--
		} else { // restart member i
			if gm.members[st.member].bin == "bin/user/kvd" {
				// For now, we don't restart kvds
				log.Printf("=== %v: kvd failed %v\n", proc.GetProgram(), gm.members[st.member].pid)
				continue
			}
			start := make(chan bool)
			go gm.members[st.member].run(st.member, start, done)
			<-start
		}
	}
	gm.ch <- true
}

func (gm *GroupMgr) Wait() {
	<-gm.ch
}

func (gm *GroupMgr) Stop() error {
	// members may not run in order of members, and blocked
	// waiting for Wlease, while the primary keeps running,
	// because it is later in the list.
	gm.stop = true
	var err error
	for _, c := range gm.members {
		go func(m *member) {
			// log.Printf("evict %v\n", m.pid)
			r := m.Evict(m.pid)
			if r != nil {
				err = r
			}
		}(c)
	}
	// log.Printf("wait for members\n")
	<-gm.ch
	log.Printf("%v: done members\n", proc.GetProgram())
	return err
}
