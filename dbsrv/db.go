package dbsrv

import (
	"path/filepath"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sessdevsrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

//
// A db proxy exporting a database server through the file system
// interface, modeled after
// http://man.cat-v.org/plan_9_contrib/4/mysqlfs
//

const (
	QDEV = "query"
)

func RunDbd(dbdaddr string) error {
	s, err := newServer(dbdaddr)
	if err != nil {
		return err
	}
	pe := proc.GetProcEnv()
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
		return err
	}
	ssrv, err := sigmasrv.NewSigmaSrvClnt(filepath.Join(sp.DB, pe.GetKernelID()), sc, s)
	if err != nil {
		return err
	}
	qd := &queryDev{dbdaddr}
	if _, err := ssrv.Create(QDEV, sp.DMDIR|0777, sp.ORDWR, sp.NoLeaseId); err != nil {
		return err
	}
	if err := sessdevsrv.NewSessDev(ssrv.MemFs, QDEV, qd.newSession, nil); err != nil {
		return err
	}
	return ssrv.RunServer()
}
