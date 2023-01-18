package uprocclnt

import (
	"path"
	"sync"

	"github.com/docker/docker/client"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/protdevclnt"
	sp "sigmaos/sigmap"
	"sigmaos/uprocsrv/proto"
)

type UprocClnt struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	pdc       *protdevclnt.ProtDevClnt
	cli       *client.Client
	container string
}

var mu sync.Mutex
var upc *UprocClnt

func MakeUProc(fsl *fslib.FsLib, pclnt *procclnt.ProcClnt, uproc *proc.Proc, realm string) error {
	// XXX don't hold global lock over RPC
	mu.Lock()
	defer mu.Unlock()
	if upc == nil {
		// Spawn uprocd in side of docker container
		u := &UprocClnt{}
		u.FsLib = fsl
		u.ProcClnt = pclnt
		program := "uprocd"
		args := []string{realm}
		pid := proc.Tpid(program + "-" + proc.GenPid().String())
		p := proc.MakePrivProcPid(pid, program, args, true)

		if err := u.SpawnContainer(p, fslib.Named(), realm); err != nil {
			return err
		}

		// XXX don't hard code
		p.AppendEnv("PATH", "/home/sigmaos/bin/user:/home/sigmaos/bin/kernel")
		p.FinalizeEnv("NONE")

		cli, c, err := container.MkContainer(p, realm)
		if err != nil {
			return nil
		}

		u.cli = cli
		u.container = c
		db.DPrintf(db.CONTAINER, "container %s\n", u.container[:10])
		pclnt.WaitStart(p.GetPid())
		db.DPrintf(db.CONTAINER, "container started %s\n", u.container[:10])
		pn := path.Join(sp.PROCD, "~local", sp.UPROCDREL)
		pdc, err := protdevclnt.MkProtDevClnt(u.FsLib, pn)
		if err != nil {
			return err
		}
		u.pdc = pdc
		upc = u
	}
	req := &proto.RunRequest{
		ProcProto: uproc.GetProto(),
	}
	res := &proto.RunResponse{}
	err := upc.pdc.RPC("UprocSrv.Run", req, res)
	if err != nil {
		return err
	}
	return nil
}
