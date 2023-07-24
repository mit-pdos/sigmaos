package group

//
// A group of servers with a primary and one or more backups
//

import (
	"log"
	"os"
	"path"
	"strconv"
	"sync"
	"time"

	"sigmaos/container"
	"sigmaos/crash"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/electclnt"
	"sigmaos/fslib"
	"sigmaos/fslibsrv"
	"sigmaos/memfs"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/repl"
	"sigmaos/replraft"
	"sigmaos/serr"
	"sigmaos/sesssrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	GRPRAFTCONF = "-raft-conf"
	TMP         = ".tmp"
	GRPCONF     = "-conf"
	GRPELECT    = "-elect"
	CTL         = "ctl"
)

func GrpPath(jobdir string, grp string) string {
	return path.Join(jobdir, grp)
}

func grpConfPath(jobdir, grp string) string {
	return GrpPath(jobdir, grp) + GRPCONF
}

func grpTmpConfPath(jobdir, grp string) string {
	return grpConfPath(jobdir, grp) + TMP
}

func grpElectPath(jobdir, grp string) string {
	return GrpPath(jobdir, grp) + GRPELECT
}

type Group struct {
	sync.Mutex
	jobdir string
	grp    string
	ip     string
	*sigmaclnt.SigmaClnt
	srv *sesssrv.SessSrv

	// We use an electclnt instead of a leaderclnt, since we don't
	// need epochs because the config is stored in etcd. If we lose
	// our connection to etcd & our leadership, we won't be able to
	// write the config file anyway.
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
	db.DPrintf(db.GROUP, "%v Try acquire leadership", g.grp)
	if err := g.ec.AcquireLeadership(nil); err != nil {
		db.DFatalf("AcquireLeadership in group.RunMember: %v", err)
	}
	db.DPrintf(db.GROUP, "%v Acquire leadership", g.grp)
}

func (g *Group) ReleaseLeadership() {
	if err := g.ec.ReleaseLeadership(); err != nil {
		db.DFatalf("release leadership: %v", err)
	}
	db.DPrintf(db.GROUP, "%v Release leadership", g.grp)
}

func (g *Group) waitForClusterConfig() {
	cfg := &GroupConfig{}
	if err := g.GetFileJsonWatch(grpConfPath(g.jobdir, g.grp), cfg); err != nil {
		db.DFatalf("Error wait for cluster config: %v", err)
	}
}

func WaitStarted(fsl *fslib.FsLib, job, grp string) (*GroupConfig, error) {
	_, err := fsl.GetFileWatch(GrpPath(job, grp))
	if err != nil {
		db.DPrintf(db.GROUP, "WaitStarted: GetFileWatch %s err %v\n", GrpPath(job, grp), err)
		return nil, err
	}
	cfg := &GroupConfig{}
	if err := fsl.GetFileJson(grpConfPath(job, grp), cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Find out if the initial cluster has started by looking for the group config.
func (g *Group) clusterStarted() bool {
	// If the final config doesn't exist yet, the cluster hasn't started.
	if _, err := g.Stat(grpConfPath(g.jobdir, g.grp)); err != nil {
		if serr.IsErrCode(err, serr.TErrNotfound) {
			db.DPrintf(db.GROUP, "found conf path %v", grpConfPath(g.jobdir, g.grp))
			return false
		}
	} else {
		db.DPrintf(db.GROUP, "didn't find conf path %v: %v", grpConfPath(g.jobdir, g.grp), err)
		// We don't expect any other errors
		if err != nil {
			db.DFatalf("Unexpected cluster config error: %v", err)

		}
	}
	// Config found.
	return true
}

func (g *Group) registerInTmpConfig() (int, *GroupConfig, *replraft.RaftConfig) {
	return g.registerInConfig(grpTmpConfPath(g.jobdir, g.grp), true)
}

func (g *Group) registerInClusterConfig() (int, *GroupConfig, *replraft.RaftConfig) {
	return g.registerInConfig(grpConfPath(g.jobdir, g.grp), false)
}

// Register self as new replica in a config file.
func (g *Group) registerInConfig(path string, init bool) (int, *GroupConfig, *replraft.RaftConfig) {
	// Read the current cluster config.
	clusterCfg, _ := g.readGroupConfig(path)
	clusterCfg.SigmaAddrs = append(clusterCfg.SigmaAddrs, sp.MkTaddrs([]string{repl.PLACEHOLDER_ADDR}))
	// Prepare peer addresses for raftlib.
	clusterCfg.RaftAddrs = append(clusterCfg.RaftAddrs, g.ip+":0")
	// Get the raft replica id.
	id := len(clusterCfg.RaftAddrs)
	// Create the raft config
	raftCfg := replraft.MakeRaftConfig(id, clusterCfg.RaftAddrs, init)
	// Get the listener address selected by the raft library.
	clusterCfg.RaftAddrs[id-1] = raftCfg.ReplAddr()
	if err := g.writeGroupConfig(path, clusterCfg); err != nil {
		db.DFatalf("Error writing group config: %v", err)
	}
	return id, clusterCfg, raftCfg
}

func (g *Group) newConfig() (int, *GroupConfig, *replraft.RaftConfig) {
	cfg := &GroupConfig{}
	cfg.SigmaAddrs = append(cfg.SigmaAddrs, sp.MkTaddrs([]string{repl.PLACEHOLDER_ADDR}))
	return 1, cfg, nil
}

func (g *Group) readGroupConfig(path string) (*GroupConfig, error) {
	cfg := &GroupConfig{}
	err := g.GetFileJson(path, cfg)
	if err != nil {
		db.DPrintf(db.GROUP_ERR, "Error GetFileJson: %v", err)
		return cfg, err
	}
	db.DPrintf(db.GROUP, "readGroupConfig: %v\n", cfg)
	return cfg, nil
}

func (g *Group) writeGroupConfig(path string, cfg *GroupConfig) error {
	err := g.PutFileJsonAtomic(path, 0777, cfg)
	if err != nil {
		return err
	}
	return nil
}

func (g *Group) writeSymlink(sigmaAddrs []sp.Taddrs) {
	// Clean sigma addrs, removing placeholders...
	srvAddrs := make(sp.Taddrs, 0)
	for _, as := range sigmaAddrs {
		addrs := sp.Taddrs{}
		for _, a := range as {
			if a.Addr != repl.PLACEHOLDER_ADDR {
				addrs = append(addrs, a)
			}
		}
		if len(addrs) > 0 {
			srvAddrs = append(srvAddrs, addrs...)
		}
	}
	mnt := sp.MkMountService(srvAddrs)
	db.DPrintf(db.GROUP, "Advertise %v/%v at %v", mnt, srvAddrs, GrpPath(g.jobdir, g.grp))
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

func RunMember(jobdir, grp string, public bool) {
	g := &Group{}
	g.grp = grp
	g.isBusy = true
	sc, err := sigmaclnt.MkSigmaClnt(sp.Tuname("kv-" + proc.GetPid().String()))
	if err != nil {
		db.DFatalf("MkSigmaClnt %v\n", err)
	}
	g.SigmaClnt = sc
	g.ec, err = electclnt.MakeElectClnt(g.FsLib, grpElectPath(jobdir, grp), 0777)
	if err != nil {
		db.DFatalf("MakeElectClnt %v\n", err)
	}
	ip, err := container.LocalIP()
	if err != nil {
		db.DFatalf("group ip %v\n", err)
	}
	g.ip = ip
	g.jobdir = jobdir

	var nReplicas int
	nReplicas, err = strconv.Atoi(os.Getenv("SIGMAREPL"))
	if err != nil {
		db.DFatalf("invalid sigmarepl: %v", err)
	}

	db.DPrintf(db.GROUP, "Starting replica with replication level %v", nReplicas)

	g.Started()
	ch := make(chan struct{})
	go g.waitExit(ch)

	g.AcquireLeadership()

	var raftCfg *replraft.RaftConfig = nil
	// ID of this replica (one-indexed counter)
	var id int
	var clusterCfg *GroupConfig

	if nReplicas > 0 && !g.clusterStarted() {
		// If the final cluster config hasn't been publisherd yet, this replica is
		// part of the initial cluster. Register self as part of the initial cluster
		// in the temporary cluster config, and wait for nReplicas to register
		// themselves as well.

		db.DPrintf(db.GROUP, "Cluster hasn't started, reading temp config")
		id, clusterCfg, raftCfg = g.registerInTmpConfig()
		// If we don't yet have enough replicas to start the cluster, wait for them
		// to register themselves.
		if id < nReplicas {
			db.DPrintf(db.GROUP, "%v < %v: Wait for more replicas", id, nReplicas)
			g.ReleaseLeadership()
			// Wait for enough memebers of the original cluster to register
			// themselves, and get the updated config.
			g.waitForClusterConfig()
			g.AcquireLeadership()
			// Get the updated cluster config.
			var err error
			if clusterCfg, err = g.readGroupConfig(grpConfPath(g.jobdir, grp)); err != nil {
				db.DFatalf("Error read group config: %v", err)
			}
			raftCfg.UpdatePeerAddrs(clusterCfg.RaftAddrs)
			db.DPrintf(db.GROUP, "%v done waiting for replicas, config: %v", id, clusterCfg)
		}
	} else {
		// Register self in the cluster config, and create it, if nReplicas == 0)
		id, clusterCfg, raftCfg = g.newConfig()
		db.DPrintf(db.GROUP, "%v new cluster: %v", id, clusterCfg)
	}

	db.DPrintf(db.GROUP, "Starting replica with cluster config %v", clusterCfg)

	root := dir.MkRootDir(ctx.MkCtxNull(), memfs.MakeInode, nil)
	srv := fslibsrv.BootSrv(root, g.ip+":0", nil, nil, raftCfg, nil)
	if srv == nil {
		db.DFatalf("Bootsrv\n")
	}
	g.srv = srv

	clusterCfg.SigmaAddrs[id-1] = sp.MkTaddrs([]string{srv.MyAddr()})

	db.DPrintf(db.GROUP, "%v:%v Writing cluster config: %v at %v", grp, id, clusterCfg,
		grpConfPath(g.jobdir, grp))

	if err := g.writeGroupConfig(grpConfPath(g.jobdir, grp), clusterCfg); err != nil {
		db.DFatalf("Write final group config: %v", err)
	}

	g.writeSymlink(clusterCfg.SigmaAddrs)

	// Release leadership.
	g.ReleaseLeadership()

	crash.Crasher(g.FsLib)
	crash.Partitioner(g.srv)
	crash.NetFailer(g.srv)

	// Record performance.
	p, err := perf.MakePerf(perf.GROUP)
	if err != nil {
		db.DFatalf("MakePerf err %v\n", err)
	}
	defer p.Done()

	// g.srv.MonitorCPU(nil)

	<-ch

	db.DPrintf(db.GROUP, "%v: group done\n", proc.GetPid())

	g.Exit(proc.MakeStatus(proc.StatusEvicted))
}

// XXX move to procclnt?
func (g *Group) waitExit(ch chan struct{}) {
	for {
		err := g.WaitEvict(proc.GetPid())
		if err != nil {
			db.DPrintf(db.GROUP, "Error WaitEvict: %v", err)
			time.Sleep(time.Second)
			continue
		}
		db.DPrintf(db.GROUP, "candidate %v %v evicted\n", g, proc.GetPid().String())
		ch <- struct{}{}
	}
}
