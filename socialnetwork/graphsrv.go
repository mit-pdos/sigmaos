package socialnetwork

import (
	sp "sigmaos/sigmap"
	dbg "sigmaos/debug"
	"sigmaos/protdevsrv"
	"sigmaos/dbclnt"
	"sigmaos/cacheclnt"
	"time"
)

// YH:
// User service for social network
// for now we use sql instead of MongoDB

const (
	GRAPH_HB_FREQ = 1
)

type GraphSrv struct {
	dbc *dbclnt.DbClnt
	cachec *cacheclnt.CacheClnt
}

func RunGraphSrv(public bool, jobname string) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_GRAPH, "Creating graph service\n")
	gsrv := &GraphSrv{}
	pds, err := protdevsrv.MakeProtDevSrvPublic(sp.SOCIAL_NETWORK_GRAPH, gsrv, public)
	if err != nil {
		return err
	}
	dbc, err := dbclnt.MkDbClnt(pds.MemFs.SigmaClnt().FsLib, sp.DBD)
	if err != nil {
		return err
	}
	gsrv.dbc = dbc
	cachec, err := cacheclnt.MkCacheClnt(pds.MemFs.SigmaClnt().FsLib, jobname)
	if err != nil {
		return err
	}
	gsrv.cachec = cachec
	dbg.DPrintf(dbg.SOCIAL_NETWORK_GRAPH, "Starting graph service\n")
	go gsrv.heartBeat()
	return pds.RunServer()
}

func (gsrv *GraphSrv) heartBeat() {
	for {
		time.Sleep(time.Duration(GRAPH_HB_FREQ) * time.Second)
		dbg.DPrintf(dbg.SOCIAL_NETWORK_GRAPH, "ALIVE!\n")
	}
}
