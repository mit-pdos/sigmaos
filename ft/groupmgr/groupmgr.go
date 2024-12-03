// package groupmgr keeps n instances of the same proc running. If one
// instance (a member) of the group of n crashes, the manager starts
// another one.  Some programs use the n instances to form a Raft
// group (e.g., kvgrp); others use it in a hot-standby configuration
// (e.g., kv balancer, imageresized).
//
// There are two ways of stopping the group manager: the caller calls
// StopGroup() or the caller calls WaitGroup() (which returns when all
// members returned with an OK status).
package groupmgr

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
	"sigmaos/sigmaclnt/fslib"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	GRPMGRDIR = sp.NAMED + "grpmgr"
)

type GroupMgr struct {
	sync.Mutex
	*sigmaclnt.SigmaClnt
	members []*member
	running bool
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

func (cfg *GroupMgrConfig) Persist(fsl *fslib.FsLib) error {
	fsl.MkDir(GRPMGRDIR, 0777)
	pn := filepath.Join(GRPMGRDIR, cfg.Job)
	if err := fsl.PutFileJsonAtomic(pn, 0777, cfg); err != nil {
		return err
	}
	return nil
}

func Recover(sc *sigmaclnt.SigmaClnt) ([]*GroupMgr, error) {
	gms := make([]*GroupMgr, 0)
	sc.ProcessDir(GRPMGRDIR, func(st *sp.Tstat) (bool, error) {
		pn := filepath.Join(GRPMGRDIR, st.Name)
		cfg := &GroupMgrConfig{}
		if err := sc.GetFileJson(pn, cfg); err != nil {
			return true, err
		}
		log.Printf("cfg %v\n", cfg)
		gms = append(gms, cfg.StartGrpMgr(sc))
		return false, nil

	})
	return gms, nil
}

func (cfg *GroupMgrConfig) StartGrpMgr(sc *sigmaclnt.SigmaClnt) *GroupMgr {
	N := cfg.NReplicas
	if cfg.NReplicas == 0 {
		N = 1
	}
	gm := &GroupMgr{
		running:   true,
		SigmaClnt: sc,
	}
	gm.ch = make(chan []*proc.Status)
	gm.members = make([]*member, N)
	for i := 0; i < N; i++ {
		db.DPrintf(db.GROUPMGR, "group %v member %v", cfg.Args, i)
		gm.members[i] = newMember(sc, cfg, i)
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

func newMember(sc *sigmaclnt.SigmaClnt, cfg *GroupMgrConfig, id int) *member {
	return &member{
		SigmaClnt:      sc,
		GroupMgrConfig: cfg,
		id:             id,
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
func (gm *GroupMgr) startL(i int, done chan *procret) {
	start := make(chan error)
	go gm.members[i].runL(start, done)
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
func (gm *GroupMgr) stopMember(pr *procret) bool {
	return pr.err == nil && (pr.status.IsStatusOK() || pr.status.IsStatusEvicted() || pr.status.IsStatusFatal())
}

func (gm *GroupMgr) handleProcRet(pr *procret, gstatus *[]*proc.Status, n *int, done chan *procret) {
	// Take the lock to protect gm.running
	gm.Lock()
	defer gm.Unlock()

	if !gm.running {
		// we are finishing up; don't respawn the member
		db.DPrintf(db.GROUPMGR, "%v: done %v n %v\n", gm.members[pr.member].Program, pr.member, *n)
		*n--
	} else if gm.stopMember(pr) {
		db.DPrintf(db.GROUPMGR, "%v: stop %v\n", gm.members[pr.member].Program, pr)
		gm.running = false
		*gstatus = append(*gstatus, pr.status)
		*n--
	} else { // restart member i
		db.DPrintf(db.GROUPMGR, "%v: start %v\n", gm.members[pr.member].Program, pr)
		gm.startL(pr.member, done)
	}
}

func (gm *GroupMgr) manager(done chan *procret, n int) {
	gstatus := make([]*proc.Status, 0, n)

	for n > 0 {
		pr := <-done
		gm.handleProcRet(pr, &gstatus, &n, done)
	}
	db.DPrintf(db.GROUPMGR, "%v exit\n", gm.members[0].Program)
	for i := 0; i < len(gm.members); i++ {
		db.DPrintf(db.GROUPMGR, "%v gen# %d exit\n", gm.members[i].Program, gm.members[i].gen)
	}
	gm.ch <- gstatus
}

func (gm *GroupMgr) Crash() error {
	db.DPrintf(db.GROUPMGR, "GroupMgr Crash")
	gm.Lock()
	defer gm.Unlock()

	gm.running = false
	return nil
}

func (gm *GroupMgr) WaitGroup() []*proc.Status {
	db.DPrintf(db.GROUPMGR, "GroupMgr Wait Group")
	statuses := <-gm.ch
	db.DPrintf(db.GROUPMGR, "Done GroupMgr Wait Group")
	return statuses
}

func (gm *GroupMgr) evictGroupMembers() error {
	// Take the lock, to ensure that the group members don't change after running
	// is set to false
	gm.Lock()
	defer gm.Unlock()

	gm.running = false

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
	return err
}

// Start separate go routine to evict each member, because members may
// not run in order of members, and be blocked waiting for becoming
// leader, while the primary keeps running, because it is later in the
// list.
func (gm *GroupMgr) StopGroup() ([]*proc.Status, error) {
	db.DPrintf(db.GROUPMGR, "GroupMgr Stop")
	err := gm.evictGroupMembers()

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
