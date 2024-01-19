// memfsd is a proc that runs an in-memory file system for testing
// purposes.  Many servers use memfs for storing in-memory state and
// this allows stand-alone testing of memfs.
package main

import (
	"path"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

func main() {
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
	}
	ssrv, err := sigmasrv.NewSigmaSrvClntFence(path.Join(sp.MEMFS, sc.ProcEnv().GetPID().String()), sc)
	if err != nil {
		db.DFatalf("Error NewSigmaSrv: %v", err)
	}
	ssrv.RunServer()
}
