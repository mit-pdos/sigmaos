package srv

import (
	"path/filepath"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

//
// A db proxy exporting a database server through the file system
// interface, modeled after
// http://man.cat-v.org/plan_9_contrib/4/mysqlfs
//

func RunDbd(dbdaddr string) error {
	db.DPrintf(db.DB, "Start dbd with dbaddr %v", dbdaddr)
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
	sc.GetDialProxyClnt().AllowConnectionsFromAllRealms()
	ssrv, err := sigmasrv.NewSigmaSrvClnt(filepath.Join(sp.DB, pe.GetKernelID()), sc, s)
	if err != nil {
		return err
	}
	return ssrv.RunServer()
}
