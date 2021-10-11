package replica

import (
	"log"
	"path"

	"ulambda/fslibsrv"
	"ulambda/repl"
)

func RunMemfsdReplica(args []string, srvAddr string, unionDirPath string, config repl.Config) {
	name := path.Join(unionDirPath, config.ReplAddr())
	fss, fsl, err := fslibsrv.MakeReplMemfs(name, srvAddr, name, config)
	if err != nil {
		log.Fatalf("RunMemfdReplica: err %v\n", err)
	}
	fss.Serve()
	fsl.ExitFs(name)
}
