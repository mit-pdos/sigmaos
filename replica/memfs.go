package replica

import (
	db "ulambda/debug"
	"ulambda/fslibsrv"
	"ulambda/repl"
)

func RunMemfsdReplica(name string, config repl.Config) {
	fss, err := fslibsrv.MakeReplMemFs("INVALID", "", name, config)
	if err != nil {
		db.DFatalf("RunMemfdReplica: err %v\n", err)
	}
	fss.Serve()
	fss.Done()
}
