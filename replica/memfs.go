package replica

import (
	"log"

	"ulambda/fslibsrv"
	"ulambda/repl"
)

func RunMemfsdReplica(name string, config repl.Config) {
	fss, _, _, err := fslibsrv.MakeReplMemfs("INVALID", "", name, config)
	if err != nil {
		log.Fatalf("RunMemfdReplica: err %v\n", err)
	}
	fss.Serve()
	fss.Done()
}
