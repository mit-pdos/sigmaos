package task

import (
	"path/filepath"
	sp "sigmaos/sigmap"
)

type FtTaskSvcId string

func (id FtTaskSvcId) ServicePath() string {
	return filepath.Join(sp.NAMED, "fttask", string(id))
}

func (id FtTaskSvcId) String() string {
	return string(id)
}
