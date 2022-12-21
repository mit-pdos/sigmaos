package protdevclnt

import (
	"fmt"
	"path"
	"time"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/protdevsrv"
	rpcproto "sigmaos/protdevsrv/proto"
	"sigmaos/serr"
	"sigmaos/sessdevclnt"
	sp "sigmaos/sigmap"
)

type ProtDevClnt struct {
	*fslib.FsLib
	fd  int
	si  *protdevsrv.StatInfo
	sdc *sessdevclnt.SessDevClnt
	pn  string
}

func MkProtDevClnt(fsl *fslib.FsLib, pn string) (*ProtDevClnt, error) {
	pdc := &ProtDevClnt{FsLib: fsl, si: protdevsrv.MakeStatInfo(), pn: pn}
	sdc, err := sessdevclnt.MkSessDevClnt(pdc.FsLib, pn, protdevsrv.RPC)
	if err != nil {
		return nil, err
	}
	n, err := pdc.Open(sdc.DataPn(), sp.ORDWR)
	if err != nil {
		return nil, err
	}
	pdc.fd = n
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
	b, err = pdc.WriteRead(pdc.fd, b)
	if err != nil {
		return nil, fmt.Errorf("rpc err %v\n", err)
	}
	// Record stats (qlen not used for now).
	pdc.si.Stat(method, time.Since(start).Microseconds(), 0)

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

func (pdc *ProtDevClnt) StatsClnt() *protdevsrv.Stats {
	return pdc.si.Stats()
}

func (pdc *ProtDevClnt) StatsSrv() (*protdevsrv.Stats, error) {
	stats := &protdevsrv.Stats{}
	if err := pdc.GetFileJson(path.Join(pdc.pn, protdevsrv.STATS), stats); err != nil {
		db.DFatalf("Error getting stats")
		return nil, err
	}
	return stats, nil
}
