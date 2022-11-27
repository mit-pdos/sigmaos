package protdevclnt

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"path"
	"time"

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
	b, err := pdc.GetFile(pdc.fn + "/" + sessdev.CLONE + protdevsrv.RPC)
	if err != nil {
		return nil, fmt.Errorf("Clone err %v\n", err)
	}
	pdc.sid = "/" + string(b)
	n, err := pdc.Open(pdc.fn+pdc.sid+"/"+sessdev.DATA+protdevsrv.RPC, np.ORDWR)
	if err != nil {
		return nil, err
	}
	pdc.fd = n
	return pdc, nil
}

func (pdc *ProtDevClnt) rpc(method string, a []byte) (*protdevsrv.Reply, error) {
	req := &protdevsrv.Request{method, a}

	ab := new(bytes.Buffer)
	ae := gob.NewEncoder(ab)
	if err := ae.Encode(req); err != nil {
		return nil, err
	}
	start := time.Now()
	b, err := pdc.WriteRead(pdc.fd, ab.Bytes())
	if err != nil {
		return nil, fmt.Errorf("rpc err %v\n", err)
	}
	// Record stats (qlen not used for now).
	pdc.si.Stat(method, time.Since(start).Microseconds(), 0)
	rep := &protdevsrv.Reply{}
	rb := bytes.NewBuffer(b)
	re := gob.NewDecoder(rb)
	if err := re.Decode(rep); err != nil {
		return nil, err
	}
	return rep, nil
}

func (pdc *ProtDevClnt) RPC(method string, arg any, res any) error {
	ab := new(bytes.Buffer)
	ae := gob.NewEncoder(ab)
	if err := ae.Encode(arg); err != nil {
		return err
	}
	rep, err := pdc.rpc(method, ab.Bytes())
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
