package kv

import (
	"ulambda/fslib"
	"ulambda/groupmgr"
	"ulambda/proc"
	"ulambda/procclnt"
)

func StartBalancers(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, nbal, crashbal int, crashhelper, auto string) *groupmgr.GroupMgr {
	return groupmgr.Start(fsl, pclnt, nbal, "user/balancer", []string{crashhelper, auto}, nbal, crashbal, 0, 0)
}

func InitKeys(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt) (*KvClerk, error) {
	// Create keys
	clrk, err := MakeClerkFsl(fsl, pclnt)
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

func StartClerk(pclnt *procclnt.ProcClnt, args []string) (proc.Tpid, error) {
	p := proc.MakeProc("user/kv-clerk", args)
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
