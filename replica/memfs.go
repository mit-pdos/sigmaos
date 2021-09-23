package replica

import (
	"log"
	"path"

	db "ulambda/debug"
	"ulambda/fslibsrv"
	"ulambda/memfsd"
	"ulambda/repl"
)

type MemfsdReplica struct {
	Pid    string
	name   string
	config repl.Config
	fsd    *memfsd.Fsd
	*fslibsrv.FsLibSrv
}

func MakeMemfsdReplica(args []string, srvAddr string, unionDirPath string, config repl.Config) *MemfsdReplica {
	r := &MemfsdReplica{}
	r.Pid = args[0]
	r.config = config

	r.fsd = memfsd.MakeReplicatedFsd(srvAddr, r.config)
	r.name = path.Join(unionDirPath, config.ReplAddr())
	db.Name(r.name)
	fs, err := fslibsrv.InitFs(r.name, r.fsd, nil)
	if err != nil {
		log.Fatalf("%v: InitFs failed %v\n", args, err)
	}
	r.FsLibSrv = fs
	return r
}

func (r *MemfsdReplica) Work() {
	r.fsd.Serve()
	r.ExitFs(r.name)
}
