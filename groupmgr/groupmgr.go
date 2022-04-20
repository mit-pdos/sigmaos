package groupmgr

import (
	"fmt"
	"strconv"
	"sync/atomic"
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
	done    int32
	members []*member
	ch      chan bool
}

type member struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	pid       proc.Tpid
	bin       string
	args      []string
	crash     int
	nReplicas int
	partition int
	netfail   int
}

type procret struct {
	member int
	err    error
	status *proc.Status
}

func (pr procret) String() string {
	return fmt.Sprintf("{m %v err %v status %v}", pr.member, pr.err, pr.status)
}

func makeMember(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, bin string, args []string, crash int, nReplicas int, partition, netfail int) *member {
	return &member{fsl, pclnt, "", bin, args, crash, nReplicas, partition, netfail}
}

func (m *member) spawn() error {
	p := proc.MakeProc(m.bin, m.args)
	p.AppendEnv(proc.SIGMACRASH, strconv.Itoa(m.crash))
	p.AppendEnv(proc.SIGMAPARTITION, strconv.Itoa(m.partition))
	p.AppendEnv(proc.SIGMANETFAIL, strconv.Itoa(m.netfail))
	p.AppendEnv("SIGMAREPL", strconv.Itoa(m.nReplicas))
	if err := m.Spawn(p); err != nil {
		return err
	}
	if err := m.WaitStart(p.Pid); err != nil {
		return err
	}
	m.pid = p.Pid
	return nil
}

func (m *member) run(i int, start chan error, done chan *procret) {
	db.DPrintf("GROUPMGR", "spawn %d member %v\n", i, m.bin)
	if err := m.spawn(); err != nil {
		start <- err
		return
	}
	start <- nil
	db.DPrintf("GROUPMGR", "%v: member %d started %v\n", m.bin, i, m.pid)
	status, err := m.WaitExit(m.pid)
	db.DPrintf("GROUPMGR", "%v: member %v exited %v err %v\n", m.bin, m.pid, status, err)
	done <- &procret{i, err, status}
}

// If n == 0, run only one member, unreplicated.
// ncrash = number of group members which may crash.
func Start(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, n int, bin string, args []string, ncrash, crash, partition, netfail int) *GroupMgr {
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
		} else {
			db.DPrintf("GROUPMGR", "group %v member %v crash %v\n", args, i, crashMember)
		}
		gm.members[i] = makeMember(fsl, pclnt, bin, args, crashMember, n, partition, netfail)
	}
	done := make(chan *procret)
	starts := make([]chan error, len(gm.members))
	for i, m := range gm.members {
		start := make(chan error)
		starts[i] = start
		go m.run(i, start, done)
	}
	for _, start := range starts {
		err := <-start
		if err != nil {
			db.DFatalf("Start %v\n", err)
		}

	}
	go gm.manager(done, N)
	return gm
}

func (gm *GroupMgr) restart(i int, done chan *procret) {
	// XXX hack
	if gm.members[i].bin == "bin/user/kvd" {
		// For now, we don't restart kvds
		db.DPrintf(db.ALWAYS, "=== kvd failed %v\n", gm.members[i].pid)
		go func() {
			done <- nil
		}()
		return
	}
	start := make(chan error)
	go gm.members[i].run(i, start, done)
	err := <-start
	if err != nil {
		go func() {
			db.DPrintf("GROUPMGR_ERR", "failed to start %v: %v; try again\n", i, err)
			time.Sleep(time.Duration(10) * time.Millisecond)
			done <- &procret{i, err, nil}
		}()
	}
}

func (gm *GroupMgr) manager(done chan *procret, n int) {
	for n > 0 {
		st := <-done
		// XXX hack
		if st == nil {
			break
		}
		if atomic.LoadInt32(&gm.done) == 1 {
			db.DPrintf("GROUPMGR", "%v: done %v n %v\n", gm.members[st.member].bin, st.member, n)
			n--
		} else if st.err == nil && st.status.IsStatusOK() { // done?
			db.DPrintf("GROUPMGR", "%v: stop %v\n", gm.members[st.member].bin, st.member)
			atomic.StoreInt32(&gm.done, 1)
			n--
		} else { // restart member i
			db.DPrintf("GROUPMGR", "%v restart %v\n", gm.members[st.member].bin, st)
			gm.restart(st.member, done)
		}
	}
	db.DPrintf("GROUPMGR", "%v exit\n", gm.members[0].bin)
	gm.ch <- true
}

func (gm *GroupMgr) Wait() {
	<-gm.ch
}

func (gm *GroupMgr) Stop() error {
	// members may not run in order of members, and blocked
	// waiting for becoming leader, while the primary keeps
	// running, because it is later in the list. So, start
	// separate go routine to evict each member.
	atomic.StoreInt32(&gm.done, 1)
	var err error
	for _, c := range gm.members {
		go func(m *member) {
			db.DPrintf("GROUPMGR", "evict %v\n", m.pid)
			r := m.Evict(m.pid)
			if r != nil {
				err = r
			}
		}(c)
	}
	// log.Printf("wait for members\n")
	<-gm.ch
	db.DPrintf("GROUPMGR", "done members %v\n", gm)
	return err
}
