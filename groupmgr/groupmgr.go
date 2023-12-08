package groupmgr

import (
	"fmt"
	"log"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/pathclnt"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

//
// Keep n instances of the same program running. If one instance (a
// member) of the group of n crashes, the manager starts another one.
// Some programs use the n instances to form a Raft group (e.g.,
// kvgrp); others use it in primary-backup configuration (e.g., kv
// balancer, imageresized).
//
// There are two ways of stopping the group manager: the caller calls
// stop or the caller calls wait (which returns when the members
// returns with an OK status).
//

const (
	GRPMGRDIR = sp.NAMED + "grpmgr"
)

type GroupMgr struct {
	*sigmaclnt.SigmaClnt
	members []*member
	stop    int32
	ch      chan []*proc.Status
}

func (gm *GroupMgr) String() string {
	s := "["
	for _, m := range gm.members {
		s += fmt.Sprintf(" %v ", m)
	}
	s += "]"
	return s
}

type GroupMgrConfig struct {
	Program   string
	Args      []string
	Job       string
	Mcpu      proc.Tmcpu
	NReplicas int

	// For testing purposes
	crash     int64
	partition int64
	netfail   int64
}

// If n == 0, run only one member (i.e., no hot standby's or replication)
func NewGroupConfig(n int, bin string, args []string, mcpu proc.Tmcpu, job string) *GroupMgrConfig {
	return &GroupMgrConfig{
		NReplicas: n,
		Program:   bin,
		Args:      append([]string{job}, args...),
		Mcpu:      mcpu,
		Job:       job,
	}
}

func (cfg *GroupMgrConfig) SetTest(crash, partition, netfail int) {
	cfg.crash = int64(crash)
	cfg.partition = int64(partition)
	cfg.netfail = int64(netfail)
}

func (cfg *GroupMgrConfig) Persist(fsl *fslib.FsLib) error {
	fsl.MkDir(GRPMGRDIR, 0777)
	pn := path.Join(GRPMGRDIR, cfg.Job)
	if err := fsl.PutFileJsonAtomic(pn, 0777, cfg); err != nil {
		return err
	}
	return nil
}

func Recover(sc *sigmaclnt.SigmaClnt) ([]*GroupMgr, error) {
	gms := make([]*GroupMgr, 0)
	sc.ProcessDir(GRPMGRDIR, func(st *sp.Stat) (bool, error) {
		pn := path.Join(GRPMGRDIR, st.Name)
		cfg := &GroupMgrConfig{}
		if err := sc.GetFileJson(pn, cfg); err != nil {
			return true, err
		}
		log.Printf("cfg %v\n", cfg)
		gms = append(gms, cfg.StartGrpMgr(sc, 0))
		return false, nil

	})
	return gms, nil
}

// ncrash = number of group members which may crash.
func (cfg *GroupMgrConfig) StartGrpMgr(sc *sigmaclnt.SigmaClnt, ncrash int) *GroupMgr {
	N := cfg.NReplicas
	if cfg.NReplicas == 0 {
		N = 1
	}
	gm := &GroupMgr{SigmaClnt: sc}
	gm.ch = make(chan []*proc.Status)
	gm.members = make([]*member, N)
	for i := 0; i < N; i++ {
		crashMember := cfg.crash
		if i+1 > ncrash {
			crashMember = 0
		} else {
			db.DPrintf(db.GROUPMGR, "group %v member %v crash %v\n", cfg.Args, i, crashMember)
		}
		gm.members[i] = newMember(sc, cfg, i, crashMember)
	}
	done := make(chan *procret)
	go gm.manager(done, N)

	// make the manager start the members
	for i := 0; i < N; i++ {
		done <- &procret{i, nil, proc.NewStatusErr("start", nil)}
	}
	return gm
}

type member struct {
	*sigmaclnt.SigmaClnt
	*GroupMgrConfig
	pid    sp.Tpid
	id     int
	crash  int64
	nstart int
}

func (m *member) String() string {
	return fmt.Sprintf("{pid %v, id %d, nstart %d}", m.pid, m.id, m.nstart)
}

type procret struct {
	member int
	err    error
	status *proc.Status
}

func (pr procret) String() string {
	return fmt.Sprintf("{m %v err %v status %v}", pr.member, pr.err, pr.status)
}

func newMember(sc *sigmaclnt.SigmaClnt, cfg *GroupMgrConfig, id int, crash int64) *member {
	return &member{SigmaClnt: sc, GroupMgrConfig: cfg, crash: crash, id: id}
}

func (m *member) spawn() error {
	p := proc.NewProc(m.Program, m.Args)
	p.SetMcpu(m.Mcpu)
	p.SetCrash(m.crash)
	p.SetPartition(m.partition)
	p.SetNetFail(m.netfail)
	p.AppendEnv("SIGMAREPL", newREPL(m.id, m.NReplicas))
	// If we are specifically setting kvd's mcpu=1, then set GOMAXPROCS to 1
	// (for use when comparing to redis).
	if m.Mcpu == 1000 && strings.Contains(m.Program, "kvd") {
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

func (m *member) run(start chan error, done chan *procret) {
	db.DPrintf(db.GROUPMGR, "spawn %d member %v", m.id, m.Program)
	if err := m.spawn(); err != nil {
		start <- err
		return
	}
	start <- nil
	db.DPrintf(db.GROUPMGR, "%v: member %d started %v\n", m.Program, m.id, m.pid)
	m.nstart += 1
	status, err := m.WaitExit(m.pid)
	db.DPrintf(db.GROUPMGR, "%v: member %v exited %v err %v\n", m.Program, m.pid, status, err)
	done <- &procret{m.id, err, status}
}

func (gm *GroupMgr) start(i int, done chan *procret) {
	start := make(chan error)
	go gm.members[i].run(start, done)
	err := <-start
	if err != nil {
		go func() {
			db.DPrintf(db.GROUPMGR_ERR, "failed to start %v: %v; try again\n", i, err)
			time.Sleep(time.Duration(pathclnt.TIMEOUT) * time.Millisecond)
			done <- &procret{i, err, nil}
		}()
	}
}

// stop restarting a member?
func (gm *GroupMgr) stopMember(pr *procret) bool {
	return pr.err == nil && (pr.status.IsStatusOK() || pr.status.IsStatusEvicted() || pr.status.IsStatusFatal())
}

func (gm *GroupMgr) manager(done chan *procret, n int) {
	gstatus := make([]*proc.Status, 0, n)
	for n > 0 {
		pr := <-done
		if atomic.LoadInt32(&gm.stop) == 1 {
			// we are finishing up; don't respawn the member
			db.DPrintf(db.GROUPMGR, "%v: done %v n %v\n", gm.members[pr.member].Program, pr.member, n)
			n--
		} else if gm.stopMember(pr) {
			db.DPrintf(db.GROUPMGR, "%v: stop %v\n", gm.members[pr.member].Program, pr)
			atomic.StoreInt32(&gm.stop, 1)
			gstatus = append(gstatus, pr.status)
			n--
		} else { // restart member i
			db.DPrintf(db.GROUPMGR, "%v start %v\n", gm.members[pr.member].Program, pr)
			gm.start(pr.member, done)
		}
	}
	db.DPrintf(db.GROUPMGR, "%v exit\n", gm.members[0].Program)
	for i := 0; i < len(gm.members); i++ {
		db.DPrintf(db.GROUPMGR, "%v nstart %d exit\n", gm.members[i].Program, gm.members[i].nstart)
	}
	gm.ch <- gstatus

}

func (gm *GroupMgr) WaitGroup() []*proc.Status {
	return <-gm.ch
}

// Start separate go routine to evict each member, because members may
// not run in order of members, and be blocked waiting for becoming
// leader, while the primary keeps running, because it is later in the
// list.
func (gm *GroupMgr) StopGroup() ([]*proc.Status, error) {
	db.DPrintf(db.GROUPMGR, "GroupMgr Stop")
	atomic.StoreInt32(&gm.stop, 1)
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
	db.DPrintf(db.GROUPMGR, "wait for members")
	gstatus := <-gm.ch
	db.DPrintf(db.GROUPMGR, "done members %v %v\n", gm, gstatus)
	return gstatus, err
}

func newREPL(id, n int) string {
	return strconv.Itoa(id) + "," + strconv.Itoa(n)
}

func ParseREPL(s string) (int, int, error) {
	n := strings.Split(s, ",")
	if len(n) != 2 {
		return 0, 0, fmt.Errorf("ParseREPL len %d", len(n))
	}
	id, err := strconv.Atoi(n[0])
	if err != nil {
		return 0, 0, err
	}
	repl, err := strconv.Atoi(n[1])
	if err != nil {
		return 0, 0, err
	}
	return id, repl, nil
}
