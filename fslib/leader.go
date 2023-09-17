package fslib

import (
	"path"

	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

func (fsl *FsLib) CreateLeaderFile(pn string, b []byte, lid sp.TleaseId, f sp.Tfence) error {
	if err := fsl.NewDir(path.Dir(pn), 0777); err != nil && !serr.IsErrCode(err, serr.TErrExists) {
		return err
	}
	fd, err := fsl.CreateEphemeral(pn, 0777|sp.DMDEVICE, sp.OWRITE, lid, f)
	if err != nil {
		return err
	}
	if _, err := fsl.WriteFence(fd, b, f); err != nil {
		return err
	}
	return fsl.Close(fd)
}
