// Package sessdevclnt creates a session with a server that exports a
// session device file; see [sessdevsrv] and [clonedev]. The client
// can then read/write the DATA device file in the session's directory
// at the server. [sigmarpcchan] uses it to send/receive RPCs to a
// server.
package clnt

import (
	"path/filepath"

	db "sigmaos/debug"
	"sigmaos/sigmaclnt/fslib"
	"sigmaos/sessdev"
)

type SessDevClnt struct {
	*fslib.FsLib
	sid  string
	pn   string
	ctl  string
	data string
}

func NewSessDevClnt(fsl *fslib.FsLib, pn string) (*SessDevClnt, error) {
	sdc := &SessDevClnt{FsLib: fsl, pn: pn}

	clone := sdc.pn + "/" + sessdev.CLONE
	db.DPrintf(db.SESSDEVCLNT, "NewSessDevClnt: %q\n", clone)
	b, err := sdc.GetFile(clone)
	if err != nil {
		return nil, err
	}
	sdc.sid = string(b)
	sdc.ctl = filepath.Join(sdc.pn, sdc.sid, sessdev.CTL)
	sdc.data = filepath.Join(sdc.pn, sdc.sid, sessdev.DATA)
	return sdc, nil
}

func (sdc *SessDevClnt) CtlPn() string {
	return sdc.ctl
}

func (sdc *SessDevClnt) DataPn() string {
	return sdc.data
}
