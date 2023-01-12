package sessdevclnt

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/sessdev"
)

type SessDevClnt struct {
	*fslib.FsLib
	sid  string
	pn   string
	fn   string
	ctl  string
	data string
}

func MkSessDevClnt(fsl *fslib.FsLib, pn string, fn string) (*SessDevClnt, error) {
	sdc := &SessDevClnt{FsLib: fsl, pn: pn, fn: fn}

	db.DPrintf(db.SESSDEVCLNT, "MkSessDevClnt: %s\n", sdc.pn+"/"+sessdev.CloneName(sdc.fn))
	b, err := sdc.GetFile(sdc.pn + "/" + sessdev.CloneName(sdc.fn))
	if err != nil {
		return nil, fmt.Errorf("Clone err %v\n", err)
	}
	sid := string(b)
	sdc.sid = "/" + sessdev.SidName(sid, sdc.fn)
	sdc.ctl = sdc.pn + sdc.sid + "/" + sessdev.CTL
	sdc.data = sdc.pn + sdc.sid + "/" + sessdev.DataName(sdc.fn)
	return sdc, nil
}

func (sdc *SessDevClnt) CtlPn() string {
	return sdc.ctl
}

func (sdc *SessDevClnt) DataPn() string {
	return sdc.data
}
