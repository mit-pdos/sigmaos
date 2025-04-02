package fslib

import (
	"path/filepath"

	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

func (fsl *FsLib) CreateLeaderFile(pn sp.Tsigmapath, b []byte, lid sp.TleaseId, f sp.Tfence) error {
	if err := fsl.MkDir(filepath.Dir(pn), 0777); err != nil && !serr.IsErrCode(err, serr.TErrExists) {
		return err
	}
	fd, err := fsl.FileAPI.CreateLeased(pn, 0777|sp.DMDEVICE, sp.OWRITE, lid, f)
	if err != nil {
		return err
	}
	if _, err := fsl.FileAPI.WriteFence(fd, b, f); err != nil {
		return err
	}
	return fsl.CloseFd(fd)
}
