package replica

import (
	"log"

	"ulambda/fslibsrv"
	"ulambda/repl"
)

func RunMemfsdReplica(name string, config repl.Config) {
	fss, err := fslibsrv.MakeReplMemFs("INVALID", "", name, config)
	if err != nil {
		log.Fatalf("RunMemfdReplica: err %v\n", err)
	}
	fss.Serve()
	fss.Done()
}
