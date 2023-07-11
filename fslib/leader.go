package fslib

import (
	"path"

	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

func (fsl *FsLib) CreateLeaderFile(pn string, b []byte, lid sp.TleaseId) error {
	if err := fsl.MkDir(path.Dir(pn), 0777); err != nil && !serr.IsErrCode(err, serr.TErrExists) {
		return err
	}
	fd, err := fsl.CreateEphemeral(pn, 0777|sp.DMDEVICE, sp.OWRITE, lid)
	if err != nil {
		return err
	}
	if _, err := fsl.Write(fd, b); err != nil {
		return err
	}
	return fsl.Close(fd)
}
