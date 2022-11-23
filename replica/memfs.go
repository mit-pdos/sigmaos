package replica

import (
	db "sigmaos/debug"
	"sigmaos/memfssrv"
	"sigmaos/repl"
)

func RunMemfsdReplica(name string, config repl.Config) {
	fss, err := memfssrv.MakeReplMemFs("INVALID", "", name, config, nil)
	if err != nil {
		db.DFatalf("RunMemfdReplica: err %v\n", err)
	}
	fss.Serve()
	fss.Done()
}
