package imgresized

import (
	"strconv"

	"sigmaos/groupmgr"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

func StartImgd(sc *sigmaclnt.SigmaClnt, job string, workerMcpu proc.Tmcpu, workerMem proc.Tmem, persist bool) *groupmgr.GroupMgr {
	cfg := groupmgr.NewGroupConfig(1, "imgresized", []string{strconv.Itoa(0), strconv.Itoa(int(workerMcpu)), strconv.Itoa(int(workerMem))}, 0, job)
	if persist {
		cfg.Persist(sc.FsLib)
	}
	return cfg.StartGrpMgr(sc, 0)
}
