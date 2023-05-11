package protdevclnt

import (
	"fmt"
	"path"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/protdev"
	rpcproto "sigmaos/protdev/proto"
	"sigmaos/serr"
	"sigmaos/sessdevclnt"
	sp "sigmaos/sigmap"
)

type ProtDevClnt struct {
	fsls []*fslib.FsLib
	fds  []int
	si   *protdev.StatInfo
	sdc  *sessdevclnt.SessDevClnt
	pn   string
	idx  int32
}

func MkProtDevClnt(fsls []*fslib.FsLib, pn string) (*ProtDevClnt, error) {
	pdc := &ProtDevClnt{
		fsls: make([]*fslib.FsLib, 0, len(fsls)),
		fds:  make([]int, 0, len(fsls)),
		si:   protdev.MakeStatInfo(),
		pn:   pn,
	}
	sdc, err := sessdevclnt.MkSessDevClnt(fsls[0], pn, protdev.RPC)
	if err != nil {
		return nil, err
	}
	for _, fsl := range fsls {
		pdc.fsls = append(pdc.fsls, fsl)
		n, err := fsl.Open(sdc.DataPn(), sp.ORDWR)
		if err != nil {
			return nil, err
		}
		pdc.fds = append(pdc.fds, n)
	}
	return pdc, nil
}

func (pdc *ProtDevClnt) rpc(method string, a []byte) (*rpcproto.Reply, error) {
	req := rpcproto.Request{}
	req.Method = method
	req.Args = a

	b, err := proto.Marshal(&req)
	if err != nil {
		return nil, serr.MkErrError(err)
	}

	start := time.Now()
	idx := int(atomic.AddInt32(&pdc.idx, 1))
	b, err = pdc.fsls[idx%len(pdc.fsls)].WriteRead(pdc.fds[idx%len(pdc.fds)], b)
	if err != nil {
		return nil, fmt.Errorf("rpc err %v", err)
	}
	// Record stats
	pdc.si.Stat(method, time.Since(start).Microseconds())

	rep := &rpcproto.Reply{}
	if err := proto.Unmarshal(b, rep); err != nil {
		return nil, serr.MkErrError(err)
	}

	return rep, nil
}

func (pdc *ProtDevClnt) RPC(method string, arg proto.Message, res proto.Message) error {
	b, err := proto.Marshal(arg)
	if err != nil {
		return err
	}
	rep, err := pdc.rpc(method, b)
	if err != nil {
		return err
	}
	if rep.Error != "" {
		return fmt.Errorf("%s", rep.Error)
	}
	if err := proto.Unmarshal(rep.Res, res); err != nil {
		return err
	}
	return nil
}

func (pdc *ProtDevClnt) StatsClnt() map[string]*protdev.MethodStat {
	return pdc.si.Stats()
}

func (pdc *ProtDevClnt) StatsSrv() (*protdev.SigmaRPCStats, error) {
	stats := &protdev.SigmaRPCStats{}
	if err := pdc.fsls[0].GetFileJson(path.Join(pdc.pn, protdev.STATS), stats); err != nil {
		db.DFatalf("Error getting stats")
		return nil, err
	}
	return stats, nil
}
