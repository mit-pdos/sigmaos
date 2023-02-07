package uprocsrv

import (
	"net"
	"os"
	"strconv"
	"sync"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/protdevsrv"
	// sp "sigmaos/sigmap"
	"sigmaos/uprocsrv/proto"
)

type UprocSrv struct {
	mu    sync.Mutex
	ch    chan struct{}
	port  int
	extIp string
}

func RunUprocSrv(realm, scheddIp string, ptype proc.Ttype) error {
	db.DPrintf(db.UPROCD, "Run %v ip %v t %v\n", realm, scheddIp, ptype)
	ups := &UprocSrv{}
	ups.ch = make(chan struct{})
	h, _, r := net.SplitHostPort(scheddIp)
	if r != nil {
		return nil
	}
	ups.port = int(container.FPORT) + 1
	ups.extIp = h

	db.DPrintf(db.UPROCD, "%v: Run %v %v %s\n", proc.GetName(), realm, h, os.Environ())

	// The kernel will advertise the server, so pass "" as pn.
	pds, err := protdevsrv.MakeProtDevSrvPort("", container.FPORT.String(), ups)
	if err != nil {
		return err
	}
	err = pds.RunServer()
	db.DPrintf(db.UPROCD, "RunServer done %v\n", err)
	return nil
}

func (ups *UprocSrv) Run(req proto.RunRequest, res *proto.RunResult) error {
	uproc := proc.MakeProcFromProto(req.ProcProto)
	return container.RunUProc(uproc)
}

func (ups *UprocSrv) Port(req proto.PortRequest, res *proto.PortResult) error {
	ups.mu.Lock()
	defer ups.mu.Unlock()
	db.DPrintf(db.UPROCD, "ip %v port %v\n", ups.extIp, ups.port)
	res.Port = strconv.Itoa(ups.port)
	res.Ip = ups.extIp
	return nil
}
