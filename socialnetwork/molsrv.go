package socialnetwork

import (
	"sigmaos/socialnetwork/proto"
	"sigmaos/fs"
	dbg "sigmaos/debug"
	"sigmaos/protdevsrv"
	"sigmaos/rand"
	"time"
)

const (
	MOL_SERVICE_NAME = SOCIAL_NETWORK_ROOT + "MOL"
	MEANING = 42
	HB_FREQ = 2
)

type MeaningOfLife struct {
	sid string
}

func RunMoLSrv(public bool) error {
	mol := &MeaningOfLife{rand.String(8)}	
	dbg.DPrintf(dbg.SOCIAL_NETWORK, "==%v== Creating MoL service\n", mol.sid)
	pds, err := protdevsrv.MakeProtDevSrvPublic(MOL_SERVICE_NAME, mol, public)
	if err != nil {
		return err
	}
	dbg.DPrintf(dbg.SOCIAL_NETWORK, "==%v== Starting to run MoL service\n", mol.sid)
	go mol.heartBeat(HB_FREQ)
	return pds.RunServer()
}

// find meaning of life for request
func (mol *MeaningOfLife) FindMeaning(ctx fs.CtxI, req proto.MoLRequest, rep *proto.MoLResult) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK, "==%v== Find Meaning Req: %v\n", mol.sid, req)
	rep.Meaning = MEANING
	return nil
}

func (mol *MeaningOfLife) heartBeat(freq int) {
	for {
		time.Sleep(time.Duration(freq) * time.Second)
		dbg.DPrintf(dbg.SOCIAL_NETWORK, "==%v== IS ALIVE\n", mol.sid)
	}
}

