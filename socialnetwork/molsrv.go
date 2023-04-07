package socialnetwork

import (
	"sigmaos/socialnetwork/proto"
	"sigmaos/fs"
	dbg "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/protdevsrv"
	"sigmaos/rand"
	"time"
)

const (
	MEANING = 42
	HB_FREQ = 2
)

type MeaningOfLife struct {
	sid string
}

func RunMoLSrv(public bool) error {
	mol := &MeaningOfLife{rand.String(8)}	
	dbg.DPrintf(dbg.SOCIAL_NETWORK_MOL, "==%v== Creating MoL service\n", mol.sid)
	pds, err := protdevsrv.MakeProtDevSrvPublic(sp.SOCIAL_NETWORK_MOL, mol, public)
	if err != nil {
		return err
	}
	dbg.DPrintf(dbg.SOCIAL_NETWORK_MOL, "==%v== Starting to run MoL service\n", mol.sid)
	go mol.heartBeat(HB_FREQ)
	return pds.RunServer()
}

// find meaning of life for request
func (mol *MeaningOfLife) FindMeaning(ctx fs.CtxI, req proto.MoLRequest, rep *proto.MoLResult) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_MOL, "==%v== Find Meaning Req: %v\n", mol.sid, req)
	rep.Meaning = MEANING
	return nil
}

func (mol *MeaningOfLife) heartBeat(freq int) {
	for {
		time.Sleep(time.Duration(freq) * time.Second)
		dbg.DPrintf(dbg.SOCIAL_NETWORK_MOL, "==%v== IS ALIVE\n", mol.sid)
	}
}

