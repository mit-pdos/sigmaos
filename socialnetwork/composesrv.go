package socialnetwork

import (
	sp "sigmaos/sigmap"
	dbg "sigmaos/debug"
	"sigmaos/protdevsrv"
	"sigmaos/protdevclnt"
	"sigmaos/fs"
	"sigmaos/socialnetwork/proto"
	"math/rand"
	"sync"
)

// YH:
// Compose post service for social network
// No db or cache connection. 

const (
	COMPOSE_QUERY_OK = "OK"
)

type ComposeSrv struct {
	textc  *protdevclnt.ProtDevClnt
	postc  *protdevclnt.ProtDevClnt
	tlc    *protdevclnt.ProtDevClnt
	homec  *protdevclnt.ProtDevClnt
	sid    int32
	pcount int32
	mu     sync.Mutex
}

func RunComposeSrv(public bool, jobname string) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_COMPOSE, "Creating compose service\n")
	csrv := &ComposeSrv{}
	csrv.sid = rand.Int31n(536870912) // 2^29
	pds, err := protdevsrv.MakeProtDevSrvPublic(sp.SOCIAL_NETWORK_COMPOSE, csrv, public)
	if err != nil {
		return err
	}
	fsls := MakeFsLibs(sp.SOCIAL_NETWORK_POST, pds.MemFs.SigmaClnt().FsLib)
	pdc, err := protdevclnt.MkProtDevClnt(fsls, sp.SOCIAL_NETWORK_TEXT)
	if err != nil {
		return err
	}
	csrv.textc = pdc	
	pdc, err = protdevclnt.MkProtDevClnt(fsls, sp.SOCIAL_NETWORK_POST)
	if err != nil {
		return err
	}
	csrv.postc = pdc	
	pdc, err = protdevclnt.MkProtDevClnt(fsls, sp.SOCIAL_NETWORK_TIMELINE)
	if err != nil {
		return err
	}
	csrv.tlc = pdc	
	pdc, err = protdevclnt.MkProtDevClnt(fsls, sp.SOCIAL_NETWORK_HOME)
	if err != nil {
		return err
	}
	csrv.homec = pdc	
	dbg.DPrintf(dbg.SOCIAL_NETWORK_COMPOSE, "Starting compose service %v\n", csrv.sid)
	return pds.RunServer()
}

func (csrv *ComposeSrv) ComposePost(
		ctx fs.CtxI, req proto.ComposePostRequest, res *proto.ComposePostResponse) error {
	res.Ok = COMPOSE_QUERY_OK
	return nil
}

func (csrv *ComposeSrv) incCountSafe() int32 {
	csrv.mu.Lock()
	defer csrv.mu.Unlock()
	csrv.pcount++
	return csrv.pcount
}

func (csrv *ComposeSrv) getNextPostId() int64 {
	return int64(csrv.sid)*1e10 + int64(csrv.incCountSafe())
}

