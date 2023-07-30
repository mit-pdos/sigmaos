package sessdevclnt

import (
	"path"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/sessdev"
)

type SessDevClnt struct {
	*fslib.FsLib
	sid  string
	pn   string
	ctl  string
	data string
}

func MkSessDevClnt(fsl *fslib.FsLib, pn string) (*SessDevClnt, error) {
	sdc := &SessDevClnt{FsLib: fsl, pn: pn}

	clone := sdc.pn + "/" + sessdev.CLONE
	db.DPrintf(db.SESSDEVCLNT, "MkSessDevClnt: %q\n", clone)
	b, err := sdc.GetFile(clone)
	if err != nil {
		return nil, err
	}
	sdc.sid = string(b)
	sdc.ctl = path.Join(sdc.pn, sdc.sid, sessdev.CTL)
	sdc.data = path.Join(sdc.pn, sdc.sid, sessdev.DATA)
	return sdc, nil
}

func (sdc *SessDevClnt) CtlPn() string {
	return sdc.ctl
}

func (sdc *SessDevClnt) DataPn() string {
	return sdc.data
}
