package protdevclnt

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"path"
	"time"

	"google.golang.org/protobuf/proto"

	"sigmaos/clonedev"
	db "sigmaos/debug"
	"sigmaos/fslib"
	np "sigmaos/ninep"
	"sigmaos/protdevsrv"
	"sigmaos/sessdev"
)

type ProtDevClnt struct {
	*fslib.FsLib
	sid string
	fn  string
	fd  int
	si  *protdevsrv.StatInfo
}

func MkProtDevClnt(fsl *fslib.FsLib, fn string) (*ProtDevClnt, error) {
	pdc := &ProtDevClnt{}
	pdc.si = protdevsrv.MakeStatInfo()
	pdc.FsLib = fsl
	pdc.fn = fn
	b, err := pdc.GetFile(pdc.fn + "/" + clonedev.CloneName(protdevsrv.RPC))
	if err != nil {
		return nil, fmt.Errorf("Clone err %v\n", err)
	}
	sid := string(b)
	pdc.sid = "/" + clonedev.SidName(sid, protdevsrv.RPC)
	n, err := pdc.Open(pdc.fn+pdc.sid+"/"+sessdev.DataName(protdevsrv.RPC), np.ORDWR)
	if err != nil {
		return nil, err
	}
	pdc.fd = n
	return pdc, nil
}

func (pdc *ProtDevClnt) rpc(method string, a []byte, p bool) (*protdevsrv.Reply, error) {
	req := protdevsrv.Request{}
	req.Method = method
	req.Protobuf = p
	req.Args = a

	b, err := proto.Marshal(&req)
	if err != nil {
		return nil, np.MkErrError(err)
	}

	start := time.Now()
	b, err = pdc.WriteRead(pdc.fd, b)
	if err != nil {
		return nil, fmt.Errorf("rpc err %v\n", err)
	}
	// Record stats (qlen not used for now).
	pdc.si.Stat(method, time.Since(start).Microseconds(), 0)

	rep := &protdevsrv.Reply{}
	if err := proto.Unmarshal(b, rep); err != nil {
		return nil, np.MkErrError(err)
	}

	return rep, nil
}

func (pdc *ProtDevClnt) RPCproto(method string, arg proto.Message, res proto.Message) error {
	b, err := proto.Marshal(arg)
	if err != nil {
		return err
	}
	rep, err := pdc.rpc(method, b, true)
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

func (pdc *ProtDevClnt) RPC(method string, arg any, res any) error {
	ab := new(bytes.Buffer)
	ae := gob.NewEncoder(ab)
	if err := ae.Encode(arg); err != nil {
		return err
	}
	rep, err := pdc.rpc(method, ab.Bytes(), false)
	if err != nil {
		return err
	}
	if rep.Error != "" {
		return fmt.Errorf("%s", rep.Error)
	}
	rb := bytes.NewBuffer(rep.Res)
	rd := gob.NewDecoder(rb)
	if err := rd.Decode(res); err != nil {
		return err
	}
	return nil
}

func (pdc *ProtDevClnt) StatsClnt() *protdevsrv.Stats {
	return pdc.si.Stats()
}

func (pdc *ProtDevClnt) StatsSrv() (*protdevsrv.Stats, error) {
	stats := &protdevsrv.Stats{}
	if err := pdc.GetFileJson(path.Join(pdc.fn, protdevsrv.STATS), stats); err != nil {
		db.DFatalf("Error getting stats")
		return nil, err
	}
	return stats, nil
}
