package cacheclnt

import (
	"strconv"

	"sigmaos/fslib"
	"sigmaos/group"
	"sigmaos/groupmgr"
	"sigmaos/proc"
	"sigmaos/procclnt"
)

type CacheMgr struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	job     string
	grpmgrs []*groupmgr.GroupMgr
	n       int
}

func MkCacheMgr(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, job string, n int) *CacheMgr {
	cm := &CacheMgr{}
	cm.job = job
	cm.FsLib = fsl
	cm.ProcClnt = pclnt
	cm.n = n
	return cm
}

func (cm *CacheMgr) StartCache() {
	for g := 0; g < cm.n; g++ {
		gn := group.GRP + strconv.Itoa(g)
		grpmgr := groupmgr.Start(cm.FsLib, cm.ProcClnt, 1, "user/cached", []string{gn}, cm.job, proc.Tcore(1), 0, 0, 0, 0)
		cm.grpmgrs = append(cm.grpmgrs, grpmgr)
	}
}

func (cm *CacheMgr) StopCache() error {
	for _, grpmgr := range cm.grpmgrs {
		err := grpmgr.Stop()
		if err != nil {
			return err
		}
	}
	return nil
}
