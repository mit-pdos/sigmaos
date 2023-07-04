package imgresized

import (
	"strconv"

	"sigmaos/groupmgr"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
)

func StartImgd(sc *sigmaclnt.SigmaClnt, job string, workerMcpu proc.Tmcpu) *groupmgr.GroupMgr {
	return groupmgr.Start(sc, 1, "imgresized", []string{strconv.Itoa(0), strconv.Itoa(int(workerMcpu))}, job, 0, 1, 0, 0, 0)
}
