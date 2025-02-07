// Package rpcdevclnt creates a session with a server that exports a
// session device file; see [rpcdevsrv] and [clonedev]. The client
// can then read/write the DATA device file in the session's directory
// at the server. [sigmarpcchan] uses it to send/receive RPCs to a
// server.
package clnt

import (
	"path/filepath"

	db "sigmaos/debug"
	rpcdev "sigmaos/rpc/dev"
	"sigmaos/sigmaclnt/fslib"
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

	clone := sdc.pn + "/" + rpcdev.CLONE
	db.DPrintf(db.SESSDEVCLNT, "NewSessDevClnt: %v", clone)
	b, err := sdc.GetFile(clone)
	if err != nil {
		db.DPrintf(db.SESSDEVCLNT_ERR, "NewSessDevClnt [%v] err: %v", clone, err)
		return nil, err
	}
	sdc.sid = string(b)
	sdc.ctl = filepath.Join(sdc.pn, sdc.sid, rpcdev.CTL)
	sdc.data = filepath.Join(sdc.pn, sdc.sid, rpcdev.DATA)
	return sdc, nil
}

func (sdc *SessDevClnt) CtlPn() string {
	return sdc.ctl
}

func (sdc *SessDevClnt) DataPn() string {
	return sdc.data
}
