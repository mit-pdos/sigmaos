package sessdevclnt

import (
	"fmt"

	"sigmaos/clonedev"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/sessdevsrv"
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

	db.DPrintf(db.SESSDEVCLNT, "MkSessDevClnt: %s\n", sdc.pn+"/"+clonedev.CloneName(sdc.fn))
	b, err := sdc.GetFile(sdc.pn + "/" + clonedev.CloneName(sdc.fn))
	if err != nil {
		return nil, fmt.Errorf("Clone err %v\n", err)
	}
	sid := string(b)
	sdc.sid = "/" + clonedev.SidName(sid, sdc.fn)
	sdc.ctl = sdc.pn + sdc.sid + "/" + clonedev.CTL
	sdc.data = sdc.pn + sdc.sid + "/" + sessdevsrv.DataName(sdc.fn)
	return sdc, nil
}

func (sdc *SessDevClnt) CtlPn() string {
	return sdc.ctl
}

func (sdc *SessDevClnt) DataPn() string {
	return sdc.data
}
