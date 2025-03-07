package task

import (
	"path/filepath"
	sp "sigmaos/sigmap"
)

type FtTaskSrvId string

func (id FtTaskSrvId) ServerPath() string {
	return filepath.Join(sp.NAMED, "fttask", string(id))
}