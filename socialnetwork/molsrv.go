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

// YH:
// Toy server returning meaning of life
// Will delete after complete social network implementation

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
	go mol.heartBeat()
	return pds.RunServer()
}

// find meaning of life for request
func (mol *MeaningOfLife) FindMeaning(ctx fs.CtxI, req proto.MoLRequest, rep *proto.MoLResult) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_MOL, "==%v== Find Meaning Req: %v\n", mol.sid, req)
	rep.Meaning = MEANING
	return nil
}

func (mol *MeaningOfLife) heartBeat() {
	for {
		time.Sleep(time.Duration(HB_FREQ) * time.Second)
		dbg.DPrintf(dbg.SOCIAL_NETWORK_MOL, "==%v== IS ALIVE\n", mol.sid)
	}
}

