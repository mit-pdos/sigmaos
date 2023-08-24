package kvgrp

//
// Starts a group of servers. If nrepl > 0, then the group forms a
// raft group.  Clients can wait until the group has configured using
// WaitStarted().
//

import (
	"log"
	"path"
	"sync"
	"time"

	"sigmaos/crash"
	db "sigmaos/debug"
	"sigmaos/electclnt"
	"sigmaos/fslib"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/replraft"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

const (
	GRPCONF  = "-conf"
	GRPELECT = "-elect"
	GRPSEM   = "-sem"
	KVDIR    = sp.NAMED + "kv/"
)

func JobDir(job string) string {
	return path.Join(KVDIR, job)
}

func GrpPath(jobdir string, grp string) string {
	return path.Join(jobdir, grp)
}

func grpConfPath(jobdir, grp string) string {
	return GrpPath(jobdir, grp) + GRPCONF
}

func grpElectPath(jobdir, grp string) string {
	return GrpPath(jobdir, grp) + GRPELECT
}

func grpSemPath(jobdir, grp string) string {
	return GrpPath(jobdir, grp) + GRPSEM
}

type Group struct {
	sync.Mutex
	jobdir string
	grp    string
	ip     string
	myid   int
	*sigmaclnt.SigmaClnt
	ssrv *sigmasrv.SigmaSrv

	// We use an electclnt instead of a leaderclnt, since we don't
	// need epochs because the config is stored in etcd. If we lose
	// our connection to etcd & our leadership, we won't be able to
	// write the config file anyway.  XXX still accurate?
	ec *electclnt.ElectClnt

	isBusy bool
}

func (g *Group) testAndSetBusy() bool {
	g.Lock()
	defer g.Unlock()
	b := g.isBusy
	g.isBusy = true
	return b
}

func (g *Group) clearBusy() {
	g.Lock()
	defer g.Unlock()
	g.isBusy = false
}

func (g *Group) AcquireLeadership() {
	db.DPrintf(db.KVGRP, "%v/%v Try acquire leadership", g.grp, g.myid)
	if err := g.ec.AcquireLeadership(nil); err != nil {
		db.DFatalf("AcquireLeadership in group.RunMember: %v", err)
	}
	db.DPrintf(db.KVGRP, "%v/%v Acquire leadership", g.grp, g.myid)
}

func (g *Group) ReleaseLeadership() {
	if err := g.ec.ReleaseLeadership(); err != nil {
		db.DFatalf("release leadership: %v", err)
	}
	db.DPrintf(db.KVGRP, "%v/%v Release leadership", g.grp, g.myid)
}

// For clients to wait unil a group is ready to serve
func WaitStarted(fsl *fslib.FsLib, jobdir, grp string) (*GroupConfig, error) {
	db.DPrintf(db.KVGRP, "WaitStarted: Wait for %v\n", GrpPath(jobdir, grp))
	if _, err := fsl.GetFileWatch(GrpPath(jobdir, grp)); err != nil {
		db.DPrintf(db.KVGRP, "WaitStarted: GetFileWatch %s err %v\n", GrpPath(jobdir, grp), err)
		return nil, err
	}
	cfg := &GroupConfig{}
	if err := fsl.GetFileJson(grpConfPath(jobdir, grp), cfg); err != nil {
		db.DPrintf(db.KVGRP, "WaitStarted: GetFileJson %s err %v\n", grpConfPath(jobdir, grp), err)
		return nil, err
	}
	return cfg, nil
}

func (g *Group) writeSymlink(sigmaAddrs []sp.Taddrs) {
	srvAddrs := make(sp.Taddrs, 0)
	for _, as := range sigmaAddrs {
		addrs := sp.Taddrs{}
		for _, a := range as {
			addrs = append(addrs, a)
		}
		if len(addrs) > 0 {
			srvAddrs = append(srvAddrs, addrs...)
		}
	}
	mnt := sp.MkMountService(srvAddrs)
	db.DPrintf(db.KVGRP, "Advertise %v/%v at %v", mnt, sigmaAddrs, GrpPath(g.jobdir, g.grp))
	if err := g.MkMountSymlink(GrpPath(g.jobdir, g.grp), mnt, g.ec.Lease()); err != nil {
		db.DFatalf("couldn't read replica addrs %v err %v", g.grp, err)
	}
}

func (g *Group) op(opcode, kv string) *serr.Err {
	if g.testAndSetBusy() {
		return serr.MkErr(serr.TErrRetry, "busy")
	}
	defer g.clearBusy()

	log.Printf("%v: opcode %v kv %v\n", proc.GetProgram(), opcode, kv)
	return nil
}

func RunMember(job, grp string, public bool, myid, nrepl int) {
	g := &Group{myid: myid, grp: grp, isBusy: true}
	sc, err := sigmaclnt.MkSigmaClnt(sp.Tuname("kv-" + proc.GetPid().String()))
	if err != nil {
		db.DFatalf("MkSigmaClnt %v\n", err)
	}
	g.SigmaClnt = sc
	g.jobdir = JobDir(job)
	g.ec, err = electclnt.MakeElectClnt(g.FsLib, grpElectPath(g.jobdir, grp), 0777)
	if err != nil {
		db.DFatalf("MakeElectClnt %v\n", err)
	}

	db.DPrintf(db.KVGRP, "Starting replica %d with replication level %v", g.myid, nrepl)

	g.Started()

	ch := make(chan struct{})
	go g.waitExit(ch)

	g.AcquireLeadership()

	cfg := g.readCreateCfg(g.myid, nrepl)

	var raftCfg *replraft.RaftConfig
	if nrepl > 0 {
		cfg, raftCfg = g.makeRaftCfg(cfg, g.myid, nrepl)
	}

	db.DPrintf(db.KVGRP, "Grp config: %v config: %v", g.myid, cfg)

	if err := g.startServer(cfg, raftCfg); err != nil {
		db.DFatalf("startServer %v\n", err)
	}

	g.writeSymlink(cfg.SigmaAddrs)

	g.ReleaseLeadership()

	crash.Crasher(g.FsLib)
	crash.Partitioner(g.ssrv.SessSrv)
	crash.NetFailer(g.ssrv.SessSrv)

	// Record performance.
	p, err := perf.MakePerf(perf.GROUP)
	if err != nil {
		db.DFatalf("MakePerf err %v\n", err)
	}
	defer p.Done()

	// g.srv.MonitorCPU(nil)

	<-ch

	db.DPrintf(db.KVGRP, "%v: group done\n", proc.GetPid())

	g.ssrv.SrvExit(proc.MakeStatus(proc.StatusEvicted))
}

// XXX move to procclnt?
func (g *Group) waitExit(ch chan struct{}) {
	for {
		err := g.WaitEvict(proc.GetPid())
		if err != nil {
			db.DPrintf(db.KVGRP, "Error WaitEvict: %v", err)
			time.Sleep(time.Second)
			continue
		}
		db.DPrintf(db.KVGRP, "candidate %v evicted\n", proc.GetPid())
		ch <- struct{}{}
	}
}
