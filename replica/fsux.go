package replica

import (
	"os"
	"path"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/repl"
	"ulambda/ux"
)

type FsUxReplica struct {
	Pid    string
	name   string
	mount  string
	config repl.Config
	ux     *fsux.FsUx
	*fslib.FsLib
}

func MakeFsUxReplica(args []string, srvAddr string, unionDirPath string, config repl.Config) *FsUxReplica {
	r := &FsUxReplica{}
	r.Pid = args[0]
	r.mount = "/tmp"
	r.config = config

	fsl := fslib.MakeFsLib("fsux-chain-replica" + config.ReplAddr())
	r.FsLib = fsl

	r.ux = fsux.MakeReplicatedFsUx(r.mount, srvAddr, "", config)
	r.name = path.Join(unionDirPath, config.ReplAddr())
	// Post in union dir
	err := r.PostService(srvAddr, r.name)
	if err != nil {
		db.DFatalf("PostService %v error: %v", r.name, err)
	}
	return r
}

func (r *FsUxReplica) setupMountPoint() {
	r.mount = "/tmp/fsux-" + r.config.ReplAddr()
	// Remove the old mount if it already existed
	os.RemoveAll(r.mount)
	os.Mkdir(r.mount, 0777)
}

func (r *FsUxReplica) Work() {
	r.ux.Serve()
}
