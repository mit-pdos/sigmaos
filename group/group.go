package group

//
// A group of servers with a primary and one or more backups
//

import (
	"errors"
	"log"
	"os"
	"path"
	"strconv"
	"sync"

	"sigmaos/container"
	"sigmaos/crash"
	db "sigmaos/debug"
	"sigmaos/electclnt"
	"sigmaos/memfssrv"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/repl"
	"sigmaos/replraft"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	_GRPDIR      = "group"
	GRP          = "grp-"
	GRPRAFTCONF  = "-raft-conf"
	TMP          = ".tmp"
	GRPCONF      = "-conf"
	GRPELECT     = "-elect"
	GRPCONFNXT   = "-conf-next"
	GRPCONFNXTBK = GRPCONFNXT + "#"
	CTL          = "ctl"
)

func JobDir(jobdir string) string {
	return path.Join(jobdir, _GRPDIR)
}

func GrpPath(jobdir string, grp string) string {
	return path.Join(JobDir(jobdir), grp)
}

func GrpSym(jobdir, grp string) string {
	return GrpPath(jobdir, grp)
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

func grpConfNxt(jobdir, grp string) string {
	return GrpPath(jobdir, grp) + GRPCONFNXT
}

func grpConfNxtBk(jobdir, grp string) string {
	return GrpPath(jobdir, grp) + GRPCONFNXTBK
}

type Group struct {
	sync.Mutex
	jobdir string
	grp    string
	ip     string
	*sigmaclnt.SigmaClnt
	ec     *electclnt.ElectClnt // We use an electclnt instead of an epochclnt because the config is stored in named. If we lose our connection to named & our leadership, we won't be able to write the config file anyway.
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

// Find out if the initial cluster has started by looking for the group config.
func (g *Group) clusterStarted() bool {
	// If the final config doesn't exist yet, the cluster hasn't started.
	if _, err := g.Stat(grpConfPath(g.jobdir, g.grp)); err != nil {
		var serr *serr.Err
		if errors.As(err, &serr) && serr.IsErrNotfound() {
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
	db.DPrintf(db.GROUP, "Advertise %v", srvAddrs)
	mnt := sp.MkMountService(srvAddrs)
	if err := g.MkMountSymlink(GrpSym(g.jobdir, g.grp), mnt); err != nil {
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
	sc, err := sigmaclnt.MkSigmaClnt("kv-" + proc.GetPid().String())
	if err != nil {
		db.DFatalf("MkSigmaClnt %v\n", err)
	}
	g.SigmaClnt = sc
	g.ec = electclnt.MakeElectClnt(g.FsLib, grpElectPath(jobdir, grp), 0777)
	ip, err := container.LocalIP()
	if err != nil {
		db.DFatalf("group ip %v\n", err)
	}
	g.ip = ip
	g.jobdir = jobdir

	crash.Crasher(g.FsLib)

	// XXX need this?
	g.MkDir(_GRPDIR, 0777)
	g.MkDir(JobDir(jobdir), 0777)

	var nReplicas int
	nReplicas, err = strconv.Atoi(os.Getenv("SIGMAREPL"))
	if err != nil {
		db.DFatalf("invalid sigmarepl: %v", err)
	}

	db.DPrintf(db.GROUP, "Starting replica with replication level %v", nReplicas)

	g.AcquireLeadership()

	var raftCfg *replraft.RaftConfig = nil
	// ID of this replica (one-indexed counter)
	var id int
	var clusterCfg *GroupConfig

	// If running replicated...
	if nReplicas > 0 {
		// If the final cluster config hasn't been publisherd yet, this replica is
		// part of the initial cluster. Register self as part of the initial cluster
		// in the temporary cluster config, and wait for nReplicas to register
		// themselves as well.
		if !g.clusterStarted() {
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
			// Register self in the cluster config.
			id, clusterCfg, raftCfg = g.registerInClusterConfig()
			db.DPrintf(db.GROUP, "%v cluster already started: %v", id, clusterCfg)
		}
	}

	db.DPrintf(db.GROUP, "Starting replica with cluster config %v", clusterCfg)

	// start server but don't publish its existence
	var mfs *memfssrv.MemFs
	var err1 error
	if public {
		mfs, err1 = memfssrv.MakeReplMemFsFslPublic("", g.SigmaClnt, raftCfg, proc.GetRealm())
	} else {
		mfs, err1 = memfssrv.MakeReplMemFsFsl(g.ip+":0", "", g.SigmaClnt, raftCfg)
	}
	if err1 != nil {
		db.DFatalf("StartMemFs %v\n", err1)
	}

	crash.Partitioner(mfs.SessSrv)
	crash.NetFailer(mfs.SessSrv)

	sigmaAddrs := make([]sp.Taddrs, 0)

	// If running replicated...
	if nReplicas > 0 {
		// Get the final sigma addr
		if public {
			clusterCfg.SigmaAddrs[id-1] = mfs.MyAddrsPublic(proc.GetNet())
		} else {
			clusterCfg.SigmaAddrs[id-1] = sp.MkTaddrs([]string{mfs.MyAddr()})
		}
		db.DPrintf(db.GROUP, "%v:%v Writing cluster config: %v", grp, id, clusterCfg)

		if err := g.writeGroupConfig(grpConfPath(g.jobdir, grp), clusterCfg); err != nil {
			db.DFatalf("Write final group config: %v", err)
		}
		sigmaAddrs = clusterCfg.SigmaAddrs
	} else {
		if public {
			sigmaAddrs = append(sigmaAddrs, mfs.MyAddrsPublic(proc.GetNet()))
		} else {
			sigmaAddrs = append(sigmaAddrs, sp.MkTaddrs([]string{mfs.MyAddr()}))
		}
	}

	g.writeSymlink(sigmaAddrs)

	// Release leadership.
	g.ReleaseLeadership()

	// Record performance.
	p, err := perf.MakePerf(perf.GROUP)
	if err != nil {
		db.DFatalf("MakePerf err %v\n", err)
	}
	defer p.Done()

	mfs.MonitorCPU(nil)

	mfs.Serve()
	mfs.Done()
}
