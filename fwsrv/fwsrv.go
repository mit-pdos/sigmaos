package fwsrv

import (
	db "sigmaos/debug"
	// "sigmaos/serr"
	"sigmaos/fwsrv/proto"
	"sigmaos/protdevsrv"
	//	sp "sigmaos/sigmap"
)

type FwSrv struct {
}

// XXX should open the Address to the outside world, if allowed
func (fw *FwSrv) Announce(req proto.AnnounceRequest, rep *proto.AnnounceResult) error {
	db.DPrintf(db.FW, "announce %v\n", req.Address)
	rep.Ok = true
	return nil
}

func RunFireWall() error {
	// seccomp.LoadFilter()  // sanity check: if enabled we want dbd to fail
	fwsrv := &FwSrv{}
	pds, err := protdevsrv.MakeProtDevSrv("XXX", fwsrv)
	if err != nil {
		return err
	}
	return pds.RunServer()
}
