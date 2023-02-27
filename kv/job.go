package kv

import (
	"path"
	"strconv"

	"sigmaos/fslib"
	"sigmaos/group"
	"sigmaos/groupmgr"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/rand"
	"sigmaos/semclnt"
	"sigmaos/sigmaclnt"
	"sigmaos/test"
)

const (
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

// TODO make grpdir a subdir of this job.
func kvShardPath(job, kvd string, shard Tshard) string {
	return path.Join(group.GrpPath(JobDir(job), kvd), "shard"+shard.String())
}

type KVJob struct {
	*sigmaclnt.SigmaClnt
	nkvd     int        // Number of kvd groups to run the test with.
	kvdrepl  int        // kvd replication level
	kvdncore proc.Tcore // Number of exclusive cores allocated to each kvd.
	ck       *KvClerk   // A clerk which can be used for initialization.
	auto     string     // Balancer auto-balancing setting.
	job      string
	ready    chan bool
	sem      *semclnt.SemClnt
	sempath  string
	balgm    *groupmgr.GroupMgr
	kvdgms   []*groupmgr.GroupMgr
	cpids    []proc.Tpid
}

func MakeKvdJob(sc *sigmaclnt.SigmaClnt, nkvd int, kvdrepl int, kvdncore proc.Tcore, auto string) (*KVJob, error) {
	kvj := &KVJob{}
	kvj.SigmaClnt = sc
	kvj.nkvd = nkvd
	kvj.kvdrepl = kvdrepl
	kvj.kvdncore = kvdncore
	kvj.job = rand.String(16)
	kvj.auto = auto
	kvj.ready = make(chan bool)

	// May already exit
	kvj.MkDir(KVDIR, 0777)
	// Should not exist.
	if err := kvj.MkDir(JobDir(kvj.job), 0777); err != nil {
		return nil, err
	}

	kvj.sempath = path.Join(JobDir(kvj.job), "kvclerk-sem")
	kvj.sem = semclnt.MakeSemClnt(kvj.FsLib, kvj.sempath)
	if err := kvj.sem.Init(0); err != nil {
		return nil, err
	}
	kvj.kvdgms = []*groupmgr.GroupMgr{}
	kvj.cpids = []proc.Tpid{}
	return kvj, nil
}

func (kvj *KVJob) Job() string {
	return kvj.job
}

func (kvj *KVJob) StartJob() error {
	kvj.balgm = StartBalancers(kvj.FsLib, kvj.ProcClnt, kvj.job, NBALANCER, 0, kvj.kvdncore, "0", kvj.auto)
	// Add an initial kvd group to put keys in.
	return kvj.AddKVDGroup()
}

func (kvj *KVJob) AddKVDGroup() error {
	// Name group
	grp := group.GRP + strconv.Itoa(len(kvj.kvdgms))
	// Spawn group
	kvj.kvdgms = append(kvj.kvdgms, SpawnGrp(kvj.FsLib, kvj.ProcClnt, kvj.job, grp, kvj.kvdncore, kvj.kvdrepl, 0))
	// Get balancer to add the group
	if err := BalancerOpRetry(kvj.FsLib, kvj.job, "add", grp); err != nil {
		return err
	}
	return nil
}

func (kvj *KVJob) RemoveKVDGroup() error {
	n := len(kvj.kvdgms) - 1
	// Get group nambe
	grp := group.GRP + strconv.Itoa(n)
	// Get balancer to remove the group
	if err := BalancerOpRetry(kvj.FsLib, kvj.job, "del", grp); err != nil {
		return err
	}
	// Stop kvd group
	if err := kvj.kvdgms[n].Stop(); err != nil {
		return err
	}
	// Remove kvd group
	kvj.kvdgms = kvj.kvdgms[:n]
	return nil
}

func (kvj *KVJob) Stop() error {
	nkvds := len(kvj.kvdgms)
	for i := 0; i < nkvds-1; i++ {
		kvj.RemoveKVDGroup()
	}
	// Stop the balancers.
	kvj.balgm.Stop()
	// Remove the last kvd group after removing the balancer.
	kvj.kvdgms[0].Stop()
	kvj.kvdgms = nil
	if err := RemoveJob(kvj.FsLib, kvj.job); err != nil {
		return err
	}
	return nil
}

func StartBalancers(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, jobname string, nbal, crashbal int, kvdncore proc.Tcore, crashhelper, auto string) *groupmgr.GroupMgr {
	kvdnc := strconv.Itoa(int(kvdncore))
	return groupmgr.Start(fsl, pclnt, nbal, "balancer", []string{crashhelper, kvdnc, auto}, jobname, 0, nbal, crashbal, 0, 0)
}

func SpawnGrp(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, jobname, grp string, ncore proc.Tcore, repl, ncrash int) *groupmgr.GroupMgr {
	return groupmgr.Start(fsl, pclnt, repl, "kvd", []string{grp, strconv.FormatBool(test.Overlays)}, JobDir(jobname), ncore, ncrash, CRASHKVD, 0, 0)
}

func InitKeys(sc *sigmaclnt.SigmaClnt, job string, nkeys int) (*KvClerk, error) {
	// Create keys
	clrk, err := MakeClerkFsl(sc, job)
	if err != nil {
		return nil, err
	}
	for i := uint64(0); i < uint64(nkeys); i++ {
		err := clrk.Put(MkKey(i), []byte{})
		if err != nil {
			return clrk, err
		}
	}
	return clrk, nil
}

func StartClerk(pclnt *procclnt.ProcClnt, job string, args []string, ncore proc.Tcore) (proc.Tpid, error) {
	args = append([]string{job}, args...)
	p := proc.MakeProc("kv-clerk", args)
	p.SetNcore(ncore)
	// SpawnBurst to spread clerks across procds.
	_, errs := pclnt.SpawnBurst([]*proc.Proc{p})
	if len(errs) > 0 {
		return p.GetPid(), errs[0]
	}
	err := pclnt.WaitStart(p.GetPid())
	return p.GetPid(), err
}

func StopClerk(pclnt *procclnt.ProcClnt, pid proc.Tpid) (*proc.Status, error) {
	err := pclnt.Evict(pid)
	if err != nil {
		return nil, err
	}
	status, err := pclnt.WaitExit(pid)
	return status, err
}

func RemoveJob(fsl *fslib.FsLib, job string) error {
	return fsl.RmDir(JobDir(job))
}
