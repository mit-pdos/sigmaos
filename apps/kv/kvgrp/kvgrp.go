// The kvgrp package starts a group of servers. If nrepl > 0, then the
// servers form a Raft group.  If nrepl == 0, then group is either a
// single server with possibly some backup servers. Clients can wait
// until the group has configured using WaitStarted().
package kvgrp

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"sigmaos/apps/kv/repl/raft"
	db "sigmaos/debug"
	"sigmaos/ft/leaderclnt"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/util/crash"
	"sigmaos/util/perf"
)

const (
	GRPCONF  = "-conf"
	GRPELECT = "-elect"
	GRPSEM   = "-sem"
	KVDIR    = sp.NAMED + "kv/"

	CRASH = 1000
)

func JobDir(job string) string {
	return filepath.Join(KVDIR, job)
}

func GrpPath(jobdir string, grp string) string {
	return filepath.Join(jobdir, grp)
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
	gen    int
	*sigmaclnt.SigmaClnt
	ssrv   *sigmasrv.SigmaSrv
	lc     *leaderclnt.LeaderClnt
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
	if err := g.lc.LeadAndFence(nil, []string{g.jobdir}); err != nil {
		db.DFatalf("LeadAndFence err %v", err)
	}
	db.DPrintf(db.KVGRP, "%v/%v Acquired leadership", g.grp, g.myid)
}

func (g *Group) ReleaseLeadership() {
	if err := g.lc.ReleaseLeadership(); err != nil {
		db.DFatalf("release leadership: %v", err)
	}
	db.DPrintf(db.KVGRP, "%v/%v gen# %d Released leadership", g.grp, g.myid, g.gen)
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

func (g *Group) advertiseEPs(sigmaEPs []*sp.Tendpoint) {
	srvAddrs := make([]*sp.Taddr, 0)
	for _, ep := range sigmaEPs {
		if ep != nil {
			srvAddrs = append(srvAddrs, ep.Addrs()...)
		}
	}
	ep := sp.NewEndpoint(sp.INTERNAL_EP, srvAddrs)
	db.DPrintf(db.KVGRP, "advertiseEPs: advertise %v at %v", srvAddrs, GrpPath(g.jobdir, g.grp))
	if len(sigmaEPs) == 1 {
		if err := g.MkLeasedEndpoint(GrpPath(g.jobdir, g.grp), ep, g.lc.Lease()); err != nil {
			db.DFatalf("advertiseEPs: make replica addrs file %v err %v", g.grp, err)
		}
	} else {
		if err := g.WriteEndpointFile(GrpPath(g.jobdir, g.grp), ep); err != nil {
			db.DFatalf("advertiseEPs: make replica addrs file %v err %v", g.grp, err)
		}
	}
}

func RunMember(job, grp string, myid, nrepl int) {
	g := &Group{myid: myid, grp: grp, isBusy: true}
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("NewSigmaClnt %v\n", err)
	}
	g.SigmaClnt = sc
	g.jobdir = JobDir(job)

	s := os.Getenv(proc.SIGMAGEN)
	if gen, err := strconv.Atoi(s); err == nil {
		g.gen = gen
	}

	g.lc, err = leaderclnt.NewLeaderClnt(sc.FsLib, grpElectPath(g.jobdir, grp), 0777)
	if err != nil {
		db.DFatalf("NewLeaderClnt %v\n", err)
	}

	db.DPrintf(db.KVGRP, "Starting replica %d (gen# %d) with nrepl %v", g.myid, g.gen, nrepl)

	g.Started()

	ch := make(chan struct{})
	go g.waitExit(ch)

	g.AcquireLeadership()

	cfg := g.readCreateCfg(nrepl)

	var raftCfg *raft.RaftConfig
	if nrepl > 0 {
		cfg, raftCfg = g.newRaftCfg(cfg, g.myid, nrepl)
	}

	g.updateCfg(cfg, g.lc.Fence())

	db.DPrintf(db.KVGRP, "Lock holder: %v config: %v raftCfg %v", g.myid, cfg, raftCfg)

	cfg, err = g.startServer(cfg, raftCfg)
	if err != nil {
		db.DFatalf("startServer %v\n", err)
	}

	g.advertiseEPs(cfg.SigmaEPs)

	g.ReleaseLeadership()

	if (nrepl > 0 && cfg.Fence.Seqno == 1) || nrepl == 0 {
		// if nrepl > 0, crash only on first configuration because restart
		// and reconfigure aren't supported
		crash.Failer(crash.KVD_CRASH, func(e crash.Tevent) {
			crash.Crash()
		})
		crash.Failer(crash.KVD_NETFAIL, func(e crash.Tevent) {
			g.ssrv.SessSrv.PartitionClient(false)
		})
		crash.Failer(crash.KVD_PARTITION, func(e crash.Tevent) {
			g.ssrv.SessSrv.PartitionClient(true)
		})
	}

	// Record performance.
	p, err := perf.NewPerf(g.ProcEnv(), perf.GROUP)
	if err != nil {
		db.DFatalf("NewPerf err %v\n", err)
	}
	defer p.Done()

	// g.srv.MonitorCPU(nil)

	db.DPrintf(db.KVGRP, "%v/%v: wait ch\n", g.grp, g.myid)

	<-ch

	db.DPrintf(db.KVGRP, "%v/%v: pid %v done\n", g.grp, g.myid, g.ProcEnv().GetPID())

	g.ssrv.SrvExit(proc.NewStatus(proc.StatusEvicted))
}

func (g *Group) waitExit(ch chan struct{}) {
	for {
		err := g.WaitEvict(g.ProcEnv().GetPID())
		if err != nil {
			db.DPrintf(db.KVGRP, "WaitEvict err %v", err)
			time.Sleep(time.Second)
			continue
		}
		db.DPrintf(db.KVGRP, "waitExit: %v %v evicted\n", g, g.ProcEnv().GetPID())
		ch <- struct{}{}
	}
}
