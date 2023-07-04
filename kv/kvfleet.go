package kv

import (
	"path"
	"strconv"

	"sigmaos/fslib"
	"sigmaos/group"
	"sigmaos/groupmgr"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/test"
)

const (
	GRP           = "grp-"
	NKV           = 10
	NSHARD        = 10 * NKV
	NBALANCER     = 3
	KVDIR         = "name/kv/"
	KVCONF        = "config"
	KVBALANCER    = "balancer"
	KVBALANCERCTL = "ctl"
)

func JobDir(job string) string {
	return path.Join(KVDIR, job)
}

func KVConfig(job string) string {
	return path.Join(JobDir(job), KVCONF)
}

func KVBalancer(job string) string {
	return path.Join(JobDir(job), KVBALANCER)
}

func KVBalancerCtl(job string) string {
	return path.Join(KVBalancer(job), KVBALANCERCTL)
}

func kvGrpPath(job, kvd string) string {
	return path.Join(JobDir(job), kvd)
}

func kvShardPath(job, kvd string, shard Tshard) string {
	return path.Join(kvGrpPath(job, kvd), "shard"+shard.String())
}

type KVFleet struct {
	*sigmaclnt.SigmaClnt
	nkvd        int        // Number of kvd groups to run the test with.
	kvdrepl     int        // kvd replication level
	kvdncore    proc.Tcore // Number of exclusive cores allocated to each kvd.
	ck          *KvClerk   // A clerk which can be used for initialization.
	crashhelper string     // Crash balancer helper/mover?
	auto        string     // Balancer auto-balancing setting.
	job         string
	ready       chan bool
	balgm       *groupmgr.GroupMgr
	kvdgms      []*groupmgr.GroupMgr
	cpids       []proc.Tpid
}

func MakeKvdFleet(sc *sigmaclnt.SigmaClnt, job string, nkvd int, kvdrepl int, kvdncore proc.Tcore, crashhelper, auto string) (*KVFleet, error) {
	kvf := &KVFleet{}
	kvf.SigmaClnt = sc
	kvf.nkvd = nkvd
	kvf.kvdrepl = kvdrepl
	kvf.kvdncore = kvdncore
	kvf.job = job
	kvf.crashhelper = crashhelper
	kvf.auto = auto
	kvf.ready = make(chan bool)

	// May already exit
	kvf.MkDir(KVDIR, 0777)
	// Should not exist.
	if err := kvf.MkDir(JobDir(kvf.job), 0777); err != nil {
		return nil, err
	}
	kvf.kvdgms = []*groupmgr.GroupMgr{}
	kvf.cpids = []proc.Tpid{}
	return kvf, nil
}

func (kvf *KVFleet) Job() string {
	return kvf.job
}

func (kvf *KVFleet) Nkvd() int {
	return kvf.nkvd
}

func (kvf *KVFleet) Start() error {
	kvf.balgm = startBalancers(kvf.SigmaClnt, kvf.job, NBALANCER, 0, kvf.kvdncore, kvf.crashhelper, kvf.auto)
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
	gm, err := spawnGrp(kvf.SigmaClnt, kvf.job, grp, kvf.kvdncore, kvf.kvdrepl, 0)
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
	if err := kvf.kvdgms[n].Stop(); err != nil {
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
	kvf.balgm.Stop()
	// Remove the last kvd group after removing the balancer.
	kvf.kvdgms[0].Stop()
	kvf.kvdgms = nil
	if err := RemoveJob(kvf.FsLib, kvf.job); err != nil {
		return err
	}
	return nil
}

func startBalancers(sc *sigmaclnt.SigmaClnt, job string, nbal, crashbal int, kvdncore proc.Tcore, crashhelper, auto string) *groupmgr.GroupMgr {
	kvdnc := strconv.Itoa(int(kvdncore))
	gm := groupmgr.Start(sc, nbal, "balancer", []string{crashhelper, kvdnc, auto}, job, 0, nbal, crashbal, 0, 0)
	return gm
}

func spawnGrp(sc *sigmaclnt.SigmaClnt, job, grp string, ncore proc.Tcore, repl, ncrash int) (*groupmgr.GroupMgr, error) {
	gm := groupmgr.Start(sc, repl, "kvd", []string{grp, strconv.FormatBool(test.Overlays)}, JobDir(job), ncore, ncrash, CRASHKVD, 0, 0)
	_, err := group.WaitStarted(sc.FsLib, JobDir(job), grp)
	if err != nil {
		return nil, err
	}
	return gm, nil
}

func RemoveJob(fsl *fslib.FsLib, job string) error {
	return fsl.RmDir(JobDir(job))
}
