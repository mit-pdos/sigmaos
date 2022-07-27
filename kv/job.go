package kv

import (
	"path"
	"strconv"

	"ulambda/fslib"
	"ulambda/group"
	"ulambda/groupmgr"
	"ulambda/proc"
	"ulambda/procclnt"
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

func StartBalancers(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, jobname string, nbal, crashbal int, kvdncore proc.Tcore, crashhelper, auto string) *groupmgr.GroupMgr {
	kvdnc := strconv.Itoa(int(kvdncore))
	return groupmgr.Start(fsl, pclnt, nbal, "user/balancer", []string{crashhelper, kvdnc, auto}, jobname, 0, nbal, crashbal, 0, 0)
}

func SpawnGrp(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, jobname, grp string, ncore proc.Tcore, repl, ncrash int) *groupmgr.GroupMgr {
	return groupmgr.Start(fsl, pclnt, repl, "user/kvd", []string{grp}, JobDir(jobname), ncore, ncrash, CRASHKVD, 0, 0)
}

func InitKeys(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, job string) (*KvClerk, error) {
	// Create keys
	clrk, err := MakeClerkFsl(fsl, pclnt, job)
	if err != nil {
		return nil, err
	}
	for i := uint64(0); i < NKEYS; i++ {
		err := clrk.Put(MkKey(i), []byte{})
		if err != nil {
			return clrk, err
		}
	}
	return clrk, nil
}

func StartClerk(pclnt *procclnt.ProcClnt, job string, args []string, ncore proc.Tcore) (proc.Tpid, error) {
	args = append([]string{job}, args...)
	p := proc.MakeProc("user/kv-clerk", args)
	p.SetNcore(ncore)
	// SpawnBurst to spread clerks across procds.
	_, errs := pclnt.SpawnBurst([]*proc.Proc{p})
	if len(errs) > 0 {
		return p.Pid, errs[0]
	}
	err := pclnt.WaitStart(p.Pid)
	return p.Pid, err
}

func StopClerk(pclnt *procclnt.ProcClnt, pid proc.Tpid) (*proc.Status, error) {
	err := pclnt.Evict(pid)
	if err != nil {
		return nil, err
	}
	status, err := pclnt.WaitExit(pid)
	return status, err
}
