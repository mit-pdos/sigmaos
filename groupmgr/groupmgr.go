package groupmgr

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	db "sigmaos/debug"
	"sigmaos/pathclnt"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

//
// Keeps n members running. If one member crashes, the manager starts
// another one to keep the replication level up.  There are two ways
// of stopping the group manager: the caller calls stop or the callers
// calls wait (which returns when the primary member returns with an
// OK status).
//

type GroupMgr struct {
	members []*member
	done    int32
	ch      chan bool
}

type GroupMgrConfig struct {
	*sigmaclnt.SigmaClnt
	bin       string
	args      []string
	job       string
	mcpu      proc.Tmcpu
	nReplicas int

	// For testing purposes
	crash     int
	partition int
	netfail   int
}

// If n == 0, run only one member (i.e., no hot standby's or replication)
func NewGroupConfig(sc *sigmaclnt.SigmaClnt, n int, bin string, args []string, mcpu proc.Tmcpu, job string) *GroupMgrConfig {
	return &GroupMgrConfig{
		SigmaClnt: sc,
		nReplicas: n,
		bin:       bin,
		args:      append([]string{job}, args...),
		mcpu:      mcpu,
		job:       job,
	}
}

func (cfg *GroupMgrConfig) SetTest(crash, partition, netfail int) {
	cfg.crash = crash
	cfg.partition = partition
	cfg.netfail = netfail
}

// ncrash = number of group members which may crash.
func (cfg *GroupMgrConfig) Start(ncrash int) *GroupMgr {
	N := cfg.nReplicas
	if cfg.nReplicas == 0 {
		N = 1
	}
	gm := &GroupMgr{}
	gm.ch = make(chan bool)
	gm.members = make([]*member, N)
	for i := 0; i < N; i++ {
		crashMember := cfg.crash
		if i+1 > ncrash {
			crashMember = 0
		} else {
			db.DPrintf(db.GROUPMGR, "group %v member %v crash %v\n", cfg.args, i, crashMember)
		}
		gm.members[i] = makeMember(cfg, crashMember)
	}
	done := make(chan *procret)
	go gm.manager(done, N)

	// make the manager start the members
	for i := 0; i < N; i++ {
		done <- &procret{i, nil, proc.MakeStatusErr("start", nil)}
	}
	return gm
}

type member struct {
	*GroupMgrConfig
	pid     proc.Tpid
	started bool
	crash   int
}

type procret struct {
	member int
	err    error
	status *proc.Status
}

func (pr procret) String() string {
	return fmt.Sprintf("{m %v err %v status %v}", pr.member, pr.err, pr.status)
}

func makeMember(cfg *GroupMgrConfig, crash int) *member {
	return &member{GroupMgrConfig: cfg, crash: crash}
}

func (m *member) spawn() error {
	p := proc.MakeProc(m.bin, m.args)
	p.SetMcpu(m.mcpu)
	p.AppendEnv(proc.SIGMACRASH, strconv.Itoa(m.crash))
	p.AppendEnv(proc.SIGMAPARTITION, strconv.Itoa(m.partition))
	p.AppendEnv(proc.SIGMANETFAIL, strconv.Itoa(m.netfail))
	p.AppendEnv("SIGMAREPL", strconv.Itoa(m.nReplicas))
	// If we are specifically setting kvd's mcpu=1, then set GOMAXPROCS to 1
	// (for use when comparing to redis).
	if m.mcpu == 1000 && strings.Contains(m.bin, "kvd") {
		p.AppendEnv("GOMAXPROCS", strconv.Itoa(1))
	}
	db.DPrintf(db.GROUPMGR, "SpawnBurst p %v", p)
	if _, errs := m.SpawnBurst([]*proc.Proc{p}, 1); len(errs) > 0 {
		db.DPrintf(db.GROUPMGR, "Error SpawnBurst pid %v err %v", p.GetPid(), errs[0])
		return errs[0]
	}
	if err := m.WaitStart(p.GetPid()); err != nil {
		return err
	}
	m.pid = p.GetPid()
	return nil
}

func (m *member) run(i int, start chan error, done chan *procret) {
	db.DPrintf(db.GROUPMGR, "spawn %d member %v", i, m.bin)
	if err := m.spawn(); err != nil {
		start <- err
		return
	}
	start <- nil
	db.DPrintf(db.GROUPMGR, "%v: member %d started %v\n", m.bin, i, m.pid)
	m.started = true
	status, err := m.WaitExit(m.pid)
	db.DPrintf(db.GROUPMGR, "%v: member %v exited %v err %v\n", m.bin, m.pid, status, err)
	done <- &procret{i, err, status}
}

func (gm *GroupMgr) start(i int, done chan *procret) {
	// XXX hack
	if gm.members[i].bin == "kvd" && gm.members[i].started {
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
			db.DPrintf(db.GROUPMGR_ERR, "failed to start %v: %v; try again\n", i, err)
			time.Sleep(time.Duration(pathclnt.TIMEOUT) * time.Millisecond)
			done <- &procret{i, err, nil}
		}()
	}
}

func (gm *GroupMgr) manager(done chan *procret, n int) {
	for n > 0 {
		pr := <-done
		// XXX hack
		if pr == nil {
			break
		}
		if atomic.LoadInt32(&gm.done) == 1 {
			db.DPrintf(db.GROUPMGR, "%v: done %v n %v\n", gm.members[pr.member].bin, pr.member, n)
			n--
		} else if pr.err == nil && pr.status.IsStatusOK() { // done?
			db.DPrintf(db.GROUPMGR, "%v: stop %v\n", gm.members[pr.member].bin, pr.member)
			atomic.StoreInt32(&gm.done, 1)
			n--
		} else { // restart member i
			db.DPrintf(db.GROUPMGR, "%v start %v\n", gm.members[pr.member].bin, pr)
			gm.start(pr.member, done)
		}
	}
	db.DPrintf(db.GROUPMGR, "%v exit\n", gm.members[0].bin)
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
			db.DPrintf(db.GROUPMGR, "evict %v\n", m.pid)
			r := m.Evict(m.pid)
			if r != nil {
				err = r
			}
		}(c)
	}
	// log.Printf("wait for members\n")
	<-gm.ch
	db.DPrintf(db.GROUPMGR, "done members %v\n", gm)
	return err
}
