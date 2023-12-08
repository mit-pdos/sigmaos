package kv

import (
	"path"
	"strconv"

	"sigmaos/cache"
	"sigmaos/fslib"
	"sigmaos/groupmgr"
	"sigmaos/kvgrp"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	GRP             = "grp-"
	NKVGRP          = 10
	NSHARD          = 10 * NKVGRP
	NBALANCER       = 3
	KVCONF          = "config"
	KVBALANCER      = "kv-balancer"
	KVBALANCERELECT = "kv-balancer-elect"
	KVBALANCERCTL   = "ctl"
)

func KVConfig(job string) string {
	return path.Join(kvgrp.JobDir(job), KVCONF)
}

func KVBalancer(job string) string {
	return path.Join(kvgrp.JobDir(job), KVBALANCER)
}

func KVBalancerElect(job string) string {
	return path.Join(kvgrp.JobDir(job), KVBALANCERELECT)
}

func KVBalancerCtl(job string) string {
	return path.Join(KVBalancer(job), KVBALANCERCTL)
}

func kvGrpPath(job, kvd string) string {
	return path.Join(kvgrp.JobDir(job), kvd)
}

func kvShardPath(job, kvd string, shard cache.Tshard) string {
	return path.Join(kvGrpPath(job, kvd), "shard"+shard.String())
}

type KVFleet struct {
	*sigmaclnt.SigmaClnt
	nkvd        int        // Number of kvd groups to run the test with.
	kvdrepl     int        // kvd replication level
	kvdmcpu     proc.Tmcpu // Number of exclusive cores allocated to each kvd.
	ck          *KvClerk   // A clerk which can be used for initialization.
	crashbal    int        // Crash balancer
	crashhelper string     // Crash balancer helper/mover?
	crashkvd    int
	auto        string // Balancer auto-balancing setting.
	job         string
	ready       chan bool
	balgm       *groupmgr.GroupMgr
	kvdgms      []*groupmgr.GroupMgr
	cpids       []sp.Tpid
}

func NewKvdFleet(sc *sigmaclnt.SigmaClnt, job string, crashbal, nkvd, kvdrepl, crashkvd int, kvdmcpu proc.Tmcpu, crashhelper, auto string) (*KVFleet, error) {
	kvf := &KVFleet{
		SigmaClnt:   sc,
		nkvd:        nkvd,
		kvdrepl:     kvdrepl,
		crashkvd:    crashkvd,
		kvdmcpu:     kvdmcpu,
		crashbal:    crashbal,
		job:         job,
		crashhelper: crashhelper,
		auto:        auto,
		ready:       make(chan bool),
	}

	// May already exit
	kvf.MkDir(kvgrp.KVDIR, 0777)
	// Should not exist.
	if err := kvf.MkDir(kvgrp.JobDir(kvf.job), 0777); err != nil {
		return nil, err
	}
	kvf.kvdgms = []*groupmgr.GroupMgr{}
	kvf.cpids = []sp.Tpid{}
	return kvf, nil
}

func (kvf *KVFleet) Job() string {
	return kvf.job
}

func (kvf *KVFleet) Nkvd() int {
	return kvf.nkvd
}

func (kvf *KVFleet) Start() error {
	repl := ""
	if kvf.kvdrepl > 0 {
		repl = "repl"
	}
	kvf.balgm = startBalancers(kvf.SigmaClnt, kvf.job, NBALANCER, kvf.crashbal, kvf.kvdmcpu, kvf.crashhelper, kvf.auto, repl)
	for i := 0; i < kvf.nkvd; i++ {
		if err := kvf.AddKVDGroup(); err != nil {
			return err
		}
	}
	return nil
}

func (kvf *KVFleet) AddKVDGroup() error {
	// Name group
	grp := GRP + strconv.Itoa(len(kvf.kvdgms))
	// Spawn group
	gm, err := spawnGrp(kvf.SigmaClnt, kvf.job, grp, kvf.kvdmcpu, kvf.kvdrepl, kvf.crashkvd)
	if err != nil {
		return err
	}
	kvf.kvdgms = append(kvf.kvdgms, gm)
	// Get balancer to add the group
	if err := BalancerOpRetry(kvf.FsLib, kvf.job, "add", grp); err != nil {
		return err
	}
	return nil
}

func (kvf *KVFleet) RemoveKVDGroup() error {
	n := len(kvf.kvdgms) - 1
	// Get group nambe
	grp := GRP + strconv.Itoa(n)
	// Get balancer to remove the group
	if err := BalancerOpRetry(kvf.FsLib, kvf.job, "del", grp); err != nil {
		return err
	}
	// Stop kvd group
	if _, err := kvf.kvdgms[n].StopGroup(); err != nil {
		return err
	}
	// Remove kvd group
	kvf.kvdgms = kvf.kvdgms[:n]
	return nil
}

func (kvf *KVFleet) Stop() error {
	nkvds := len(kvf.kvdgms)
	for i := 0; i < nkvds-1; i++ {
		kvf.RemoveKVDGroup()
	}
	// Stop the balancers.
	kvf.balgm.StopGroup()
	// Remove the last kvd group after removing the balancer.
	kvf.kvdgms[0].StopGroup()
	kvf.kvdgms = nil
	if err := RemoveJob(kvf.FsLib, kvf.job); err != nil {
		return err
	}
	return nil
}

func startBalancers(sc *sigmaclnt.SigmaClnt, job string, nbal, crashbal int, kvdmcpu proc.Tmcpu, crashhelper, auto, repl string) *groupmgr.GroupMgr {
	kvdnc := strconv.Itoa(int(kvdmcpu))
	cfg := groupmgr.NewGroupConfig(nbal, KVBALANCER, []string{crashhelper, kvdnc, auto, repl}, 0, job)
	cfg.SetTest(crashbal, 0, 0)
	return cfg.StartGrpMgr(sc, nbal)
}

func spawnGrp(sc *sigmaclnt.SigmaClnt, job, grp string, mcpu proc.Tmcpu, repl, ncrash int) (*groupmgr.GroupMgr, error) {
	cfg := groupmgr.NewGroupConfig(repl, "kvd", []string{grp, strconv.FormatBool(test.Overlays)}, mcpu, job)
	cfg.SetTest(CRASHKVD, 0, 0)
	gm := cfg.StartGrpMgr(sc, ncrash)
	_, err := kvgrp.WaitStarted(sc.FsLib, kvgrp.JobDir(job), grp)
	if err != nil {
		return nil, err
	}
	return gm, nil
}

func RemoveJob(fsl *fslib.FsLib, job string) error {
	return fsl.RmDir(kvgrp.JobDir(job))
}
