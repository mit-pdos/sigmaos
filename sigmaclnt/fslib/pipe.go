package fslib

import (
	sp "sigmaos/sigmap"
)

func (fl *FsLib) NewPipe(pn sp.Tsigmapath, lperm sp.Tperm) error {
	lperm = lperm | sp.DMNAMEDPIPE
	// ORDWR so that close doesn't do anything to the pipe state
	fd, err := fl.Create(pn, lperm, sp.ORDWR)
	if err != nil {
		return err
	}
	return fl.CloseFd(fd)
}
