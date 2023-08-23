package imgresized

import (
	"strconv"

	"sigmaos/groupmgr"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

func StartImgd(sc *sigmaclnt.SigmaClnt, job string, workerMcpu proc.Tmcpu) *groupmgr.GroupMgr {
	cfg := groupmgr.NewGroupConfig(1, "imgresized", []string{strconv.Itoa(0), strconv.Itoa(int(workerMcpu))}, 0, job)
	return cfg.StartGrpMgr(sc, 0)
}
