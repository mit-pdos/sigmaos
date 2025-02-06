// package procgroupmgr keeps n instances of the same proc running. If one
// instance (a member) of the group of n crashes, the manager starts
// another one.  Some programs use the n instances to form a Raft
// group (e.g., kvgrp); others use it in a hot-standby configuration
// (e.g., mr coordinator, kv balancer, imageresized).
//
// There are two ways of stopping the group manager: the caller calls
// StopGroup() or the caller calls WaitGroup() (which returns when all
// members returned with an OK status).
package procgroupmgr

import (
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"sync"
	//"github.com/sasha-s/go-deadlock"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

const (
	GRPMGRDIR = sp.NAMED + "grpmgr"
)

type ProcStatus struct {
	Nrestart int
	*proc.Status
}

type ProcGroupMgr struct {
	sync.Mutex
	*sigmaclnt.SigmaClnt
	members []*member
	running bool
	ch      chan []*ProcStatus
}

func (pgm *ProcGroupMgr) String() string {
	s := "["
	for _, m := range pgm.members {
		s += fmt.Sprintf(" %v ", m)
	}
	s += "]"
	return s
}

type ProcGroupMgrConfig struct {
	Program   string
	Args      []string
	Job       string
	Mcpu      proc.Tmcpu
	NReplicas int
}

// If n == 0, run only one member (i.e., no hot standby's or replication)
func NewProcGroupConfig(n int, bin string, args []string, mcpu proc.Tmcpu, job string) *ProcGroupMgrConfig {
	return &ProcGroupMgrConfig{
		NReplicas: n,
		Program:   bin,
		Args:      append([]string{job}, args...),
		Mcpu:      mcpu,
		Job:       job,
	}
}

func (cfg *ProcGroupMgrConfig) Persist(fsl *fslib.FsLib) error {
	fsl.MkDir(GRPMGRDIR, 0777)
	pn := filepath.Join(GRPMGRDIR, cfg.Job)
	if err := fsl.PutFileJsonAtomic(pn, 0777, cfg); err != nil {
		return err
	}
	return nil
}

func Recover(sc *sigmaclnt.SigmaClnt) ([]*ProcGroupMgr, error) {
	pgms := make([]*ProcGroupMgr, 0)
	sc.ProcessDir(GRPMGRDIR, func(st *sp.Tstat) (bool, error) {
		pn := filepath.Join(GRPMGRDIR, st.Name)
		cfg := &ProcGroupMgrConfig{}
		if err := sc.GetFileJson(pn, cfg); err != nil {
			return true, err
		}
		log.Printf("cfg %v\n", cfg)
		pgms = append(pgms, cfg.StartGrpMgr(sc))
		return false, nil

	})
	return pgms, nil
}

func (cfg *ProcGroupMgrConfig) StartGrpMgr(sc *sigmaclnt.SigmaClnt) *ProcGroupMgr {
	N := cfg.NReplicas
	if cfg.NReplicas == 0 {
		N = 1
	}
	pgm := &ProcGroupMgr{
		running:   true,
		SigmaClnt: sc,
	}
	pgm.ch = make(chan []*ProcStatus)
	pgm.members = make([]*member, N)
	for i := 0; i < N; i++ {
		db.DPrintf(db.GROUPMGR, "group %v member %v", cfg.Args, i)
		pgm.members[i] = newMember(sc, cfg, i)
	}
	done := make(chan *procret)
	go pgm.manager(done, N)

	// make the manager start the members
	for i := 0; i < N; i++ {
		done <- &procret{i, nil, proc.NewStatusErr("start", nil)}
	}
	return pgm
}

type member struct {
	*sigmaclnt.SigmaClnt
	*ProcGroupMgrConfig
	pid sp.Tpid
	id  int
	gen int
}

func (m *member) String() string {
	return fmt.Sprintf("{pid %v, id %d, gen %d}", m.pid, m.id, m.gen)
}

type procret struct {
	member int
	err    error
	status *proc.Status
}

func (pr procret) String() string {
	return fmt.Sprintf("{m %v err %v status %v}", pr.member, pr.err, pr.status)
}

func newMember(sc *sigmaclnt.SigmaClnt, cfg *ProcGroupMgrConfig, id int) *member {
	return &member{
		SigmaClnt:          sc,
		ProcGroupMgrConfig: cfg,
		id:                 id,
	}
}

// Caller holds lock
func (m *member) spawnL() error {
	p := proc.NewProc(m.Program, m.Args)
	p.SetMcpu(m.Mcpu)

	p.AppendEnv(proc.SIGMAFAIL, proc.GetSigmaFail())
	p.AppendEnv(proc.SIGMAGEN, strconv.Itoa(m.gen))
	p.AppendEnv("SIGMAREPL", newREPL(m.id, m.NReplicas))
	// If we are specifically setting kvd's mcpu=1, then set GOMAXPROCS to 1
	// (for use when comparing to redis).
	if m.Mcpu == 1000 && strings.Contains(m.Program, "kvd") {
		p.AppendEnv("GOMAXPROCS", strconv.Itoa(1))
	}
	db.DPrintf(db.GROUPMGR, "Spawn p %v", p)
	if err := m.Spawn(p); err != nil {
		db.DPrintf(db.GROUPMGR, "Error Spawn pid %v err %v", p.GetPid(), err)
		return err
	}
	db.DPrintf(db.GROUPMGR, "WaitStart p %v", p)
	if err := m.WaitStart(p.GetPid()); err != nil {
		return err
	}
	db.DPrintf(db.GROUPMGR, "Done WaitStart p %v", p)
	// Lock must be held at this point, to avoid race between restart & stop
	m.pid = p.GetPid()
	return nil
}

// Caller holds lock
func (m *member) runL(start chan error, done chan *procret) {
	m.gen += 1
	db.DPrintf(db.GROUPMGR, "spawn %d member %v gen# %d", m.id, m.Program, m.gen)
	if err := m.spawnL(); err != nil {
		start <- err
		return
	}
	start <- nil
	db.DPrintf(db.GROUPMGR, "%v: member %d started %v gen# %d\n", m.Program, m.id, m.pid, m.gen)
	status, err := m.WaitExit(m.pid)
	db.DPrintf(db.GROUPMGR, "%v: member %v exited %v err %v\n", m.Program, m.pid, status, err)
	done <- &procret{m.id, err, status}
}

// Caller holds lock
func (pgm *ProcGroupMgr) startL(i int, done chan *procret) {
	start := make(chan error)
	go pgm.members[i].runL(start, done)
	err := <-start
	if err != nil {
		go func() {
			db.DPrintf(db.GROUPMGR_ERR, "failed to start %v: %v; try again\n", i, err)
			time.Sleep(sp.Conf.Path.RESOLVE_TIMEOUT)
			done <- &procret{i, err, nil}
		}()
	}
}

// stop restarting a member?
func (pgm *ProcGroupMgr) stopMember(pr *procret) bool {
	return pr.err == nil && (pr.status.IsStatusOK() || pr.status.IsStatusEvicted() || pr.status.IsStatusFatal())
}

func (pgm *ProcGroupMgr) handleProcRet(pr *procret, gstatus *[]*ProcStatus, n *int, done chan *procret) {
	// Take the lock to protect pgm.running
	pgm.Lock()
	defer pgm.Unlock()

	if !pgm.running {
		// we are finishing up; don't respawn the member
		db.DPrintf(db.GROUPMGR, "%v: done %v n %v\n", pgm.members[pr.member].Program, pr.member, *n)
		*n--
		*gstatus = append(*gstatus, &ProcStatus{pgm.members[pr.member].gen, pr.status})
	} else if pgm.stopMember(pr) {
		db.DPrintf(db.GROUPMGR, "%v: stop %v\n", pgm.members[pr.member].Program, pr)
		pgm.running = false
		*gstatus = append(*gstatus, &ProcStatus{pgm.members[pr.member].gen, pr.status})
		*n--
	} else { // restart member i
		db.DPrintf(db.GROUPMGR, "%v: start %v\n", pgm.members[pr.member].Program, pr)
		pgm.startL(pr.member, done)
	}
}

func (pgm *ProcGroupMgr) manager(done chan *procret, n int) {
	gstatus := make([]*ProcStatus, 0, n)

	for n > 0 {
		pr := <-done
		pgm.handleProcRet(pr, &gstatus, &n, done)
	}
	db.DPrintf(db.GROUPMGR, "%v exit\n", pgm.members[0].Program)
	for i := 0; i < len(pgm.members); i++ {
		db.DPrintf(db.GROUPMGR, "%v gen# %d exit\n", pgm.members[i].Program, pgm.members[i].gen)
	}
	pgm.ch <- gstatus
}

func (pgm *ProcGroupMgr) Crash() error {
	db.DPrintf(db.GROUPMGR, "ProcGroupMgr Crash")
	pgm.Lock()
	defer pgm.Unlock()

	pgm.running = false
	return nil
}

func (pgm *ProcGroupMgr) WaitGroup() []*ProcStatus {
	db.DPrintf(db.GROUPMGR, "ProcGroupMgr Wait Group")
	statuses := <-pgm.ch
	db.DPrintf(db.GROUPMGR, "Done ProcGroupMgr Wait Group")
	return statuses
}

func (pgm *ProcGroupMgr) evictGroupMembers() error {
	// Take the lock, to ensure that the group members don't change after running
	// is set to false
	pgm.Lock()
	defer pgm.Unlock()

	pgm.running = false

	var err error
	for _, c := range pgm.members {
		go func(m *member) {
			db.DPrintf(db.GROUPMGR, "evict %v\n", m.pid)
			r := m.Evict(m.pid)
			if r != nil {
				err = r
			}
		}(c)
	}
	return err
}

// Start separate go routine to evict each member, because members may
// not run in order of members, and be blocked waiting for becoming
// leader, while the primary keeps running, because it is later in the
// list.
func (pgm *ProcGroupMgr) StopGroup() ([]*ProcStatus, error) {
	db.DPrintf(db.GROUPMGR, "ProcGroupMgr Stop")
	err := pgm.evictGroupMembers()

	db.DPrintf(db.GROUPMGR, "wait for members")
	gstatus := <-pgm.ch
	db.DPrintf(db.GROUPMGR, "done members %v %v\n", pgm, gstatus)
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
