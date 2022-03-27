package groupmgr

import (
	"log"
	"strconv"
	"time"

	db "ulambda/debug"
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
	pid   proc.Tpid
	bin   string
	args  []string
	crash int
	repl  bool
}

type procret struct {
	member int
	err    error
	status *proc.Status
}

func makeMember(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, bin string, args []string, crash int, repl bool) *member {
	return &member{fsl, pclnt, "", bin, args, crash, repl}
}

func (m *member) spawn() error {
	p := proc.MakeProc(m.bin, m.args)
	p.AppendEnv("SIGMACRASH", strconv.Itoa(m.crash))
	p.AppendEnv("SIGMAREPL", strconv.FormatBool(m.repl))
	err := m.Spawn(p)
	if err != nil {
		return err
	}
	err = m.WaitStart(p.Pid)
	m.pid = p.Pid
	return err
}

func (m *member) run(i int, start chan error, done chan procret) {
	db.DLPrintf("GROUPMGR", "spawn %d member %v\n", i, m.bin)
	if err := m.spawn(); err != nil {
		start <- err
		return
	}
	db.DLPrintf("GROUPMGR", "member %d started %v\n", i, m.pid)
	start <- nil
	status, err := m.WaitExit(m.pid)
	db.DLPrintf("GROUPMGR", "member %v exited %v err %v\n", m.pid, status, err)
	done <- procret{i, err, status}
}

// If n == 0, run only one member, unreplicated.
// ncrash = number of group members which may crash.
func Start(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, n int, bin string, args []string, ncrash, crash int) *GroupMgr {
	var N int
	if n > 0 {
		N = n
	} else {
		N = 1
	}
	gm := &GroupMgr{}
	gm.ch = make(chan bool)
	gm.members = make([]*member, N)
	for i := 0; i < N; i++ {
		crashMember := crash
		if i+1 > ncrash {
			crashMember = 0
		}
		gm.members[i] = makeMember(fsl, pclnt, bin, args, crashMember, n > 0)
	}
	done := make(chan procret)
	for i, m := range gm.members {
		start := make(chan error)
		go m.run(i, start, done)
		err := <-start
		if err != nil {
			log.Fatalf("Start: couldn't start member %v err %v\n", i, err)
		}
	}
	go gm.manager(done, N)
	return gm
}

func (gm *GroupMgr) restart(i int, done chan procret) {
	if gm.members[i].bin == "bin/user/kvd" {
		// For now, we don't restart kvds
		db.DLPrintf(db.ALWAYS, "=== kvd failed %v\n", gm.members[i].pid)
		return
	}
	for true {
		start := make(chan error)
		go gm.members[i].run(i, start, done)
		err := <-start
		if err != nil {
			db.DLPrintf(db.ALWAYS, "failed to start %v: %v; try again later\n", i, err)
			time.Sleep(time.Duration(100) * time.Millisecond)
			continue
		}
		break
	}
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
			gm.restart(st.member, done)
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
	db.DLPrintf("GROUPMGR", "done members\n")
	return err
}
