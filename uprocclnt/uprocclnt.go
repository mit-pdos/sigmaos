package uprocclnt

import (
	"fmt"
	"log"
	"path"
	"time"

	"github.com/docker/docker/client"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/protdevclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/uprocsrv/proto"
)

type UprocClnt struct {
	*fslib.FsLib
	pdc       *protdevclnt.ProtDevClnt
	cli       *client.Client
	container string
}

var upc *UprocClnt

func MakeUProc(fsl *fslib.FsLib, uproc *proc.Proc, realm string) error {
	if upc == nil {
		u := &UprocClnt{}
		u.FsLib = fsl
		cli, c, err := container.MkContainer(realm)
		if err != nil {
			return nil
		}
		u.cli = cli
		u.container = c
		upc = u
		db.DPrintf(db.CONTAINER, "container %s\n", u.container[:10])
		upc.waitContainer()
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

// XXX deduplicate with bootkernelclnt
func (upc *UprocClnt) waitContainer() error {
	const N = 100
	for i := 0; i < N; i++ {
		time.Sleep(10 * time.Millisecond)
		pn := path.Join(sp.PROCD, "~local", sp.UPROCDREL)
		pdc, err := protdevclnt.MkProtDevClnt(upc.FsLib, pn)
		if err == nil {
			log.Printf("running\n")
			upc.pdc = pdc
			return nil
		} else if serr.IsErrUnavailable(err) {
			log.Printf(".")
			continue
		} else {
			return err
		}
	}
	return fmt.Errorf("waitContainer: timeded out")
}
