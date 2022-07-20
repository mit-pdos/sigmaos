package kv

import (
	"ulambda/fslib"
	"ulambda/groupmgr"
	"ulambda/procclnt"
)

func StartBalancers(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, nbal, crashbal int, crashhelper, auto string) *groupmgr.GroupMgr {
	return groupmgr.Start(fsl, pclnt, nbal, "user/balancer", []string{crashhelper, auto}, nbal, crashbal, 0, 0)
}
