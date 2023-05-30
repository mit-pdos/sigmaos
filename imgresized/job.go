package imgresized

import (
	"strconv"

	"sigmaos/groupmgr"
	"sigmaos/sigmaclnt"
)

func StartImgd(sc *sigmaclnt.SigmaClnt, job string) *groupmgr.GroupMgr {
	return groupmgr.Start(sc, 1, "imgresized", []string{strconv.Itoa(0)}, job, 0, 1, 0, 0, 0)
}
