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

func JobDir(job string) string {
	return path.Join(_KVDIR, job)
}

func KVConfig(job string) string {
	return path.Join(JobDir(job), _KVCONF)
}

func KVBalancer(job string) string {
	return path.Join(JobDir(job), _KVBALANCER)
}

func KVBalancerCtl(job string) string {
	return path.Join(KVBalancer(job), _KVBALANCERCTL)
}

// TODO make grpdir a subdir of this job.
func kvShardPath(kvd string, shard Tshard) string {
	return group.GRPDIR + "/" + kvd + "/shard" + shard.String()
}

func StartBalancers(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, jobname string, nbal, crashbal int, kvdncore proc.Tcore, crashhelper, auto string) *groupmgr.GroupMgr {
	kvdnc := strconv.Itoa(int(kvdncore))
	return groupmgr.Start(fsl, pclnt, nbal, "user/balancer", []string{jobname, crashhelper, kvdnc, auto}, 0, nbal, crashbal, 0, 0)
}

func SpawnGrp(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, grp string, ncore proc.Tcore, repl, ncrash int) *groupmgr.GroupMgr {
	return groupmgr.Start(fsl, pclnt, repl, "user/kvd", []string{grp}, ncore, ncrash, CRASHKVD, 0, 0)
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
