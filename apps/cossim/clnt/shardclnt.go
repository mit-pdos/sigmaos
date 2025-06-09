package clnt

import (
	"sync"

	"sigmaos/apps/cossim"
	"sigmaos/apps/epcache"
	epcacheclnt "sigmaos/apps/epcache/clnt"
	epcacheproto "sigmaos/apps/epcache/proto"
	db "sigmaos/debug"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

type CosSimShardClnt struct {
	mu         sync.Mutex
	fsl        *fslib.FsLib
	clnts      map[string]*CosSimClnt
	clntsSlice []*CosSimClnt
	epcc       *epcacheclnt.EndpointCacheClnt
	lastEPV    epcache.Tversion
	done       bool
}

// TODO: Add option to lazily init?
func NewCosSimShardClnt(fsl *fslib.FsLib, epcc *epcacheclnt.EndpointCacheClnt) (*CosSimShardClnt, error) {
	cssc := &CosSimShardClnt{
		fsl:     fsl,
		clnts:   make(map[string]*CosSimClnt),
		epcc:    epcc,
		lastEPV: epcache.NO_VERSION,
		done:    false,
	}
	go cssc.monitorShards()
	return cssc, nil
}

func (cssc *CosSimShardClnt) GetClnt(srvID string) (*CosSimClnt, error) {
	cssc.mu.Lock()
	defer cssc.mu.Unlock()

	clnt, ok := cssc.clnts[srvID]
	if ok {
		return clnt, nil
	}
	clnt, err := NewCosSimClnt(cssc.fsl, cssc.epcc, srvID)
	if err != nil {
		return nil, err
	}
	cssc.clnts[srvID] = clnt
	return clnt, nil
}

func (cssc *CosSimShardClnt) monitorShards() {
	for !cssc.done {
		// Get EPs
		instances, v, err := cssc.epcc.GetEndpoints(cossim.COSSIM, cssc.lastEPV)
		if err != nil {
			db.DPrintf(db.COSSIMCLNT_ERR, "Err GetEndpoints: %v", err)
			continue
		}
		// Update set of available clients
		cssc.addClnts(instances)
		// Update last endpoint version
		cssc.lastEPV = v
	}
}

func (cssc *CosSimShardClnt) addClnts(instances []*epcacheproto.Instance) {
	cssc.mu.Lock()
	defer cssc.mu.Unlock()

	for _, i := range instances {
		// If client already exists, move along
		if _, ok := cssc.clnts[i.ID]; ok {
			continue
		}
		ep := sp.NewEndpointFromProto(i.EndpointProto)
		clnt, err := NewCosSimClntFromEP(ep)
		if err != nil {
			db.DPrintf(db.COSSIMCLNT_ERR, "Err new cossim clnt: %v", err)
		}
		cssc.clnts[i.ID] = clnt
		cssc.clntsSlice = append(cssc.clntsSlice, clnt)
	}
}

func (cssc *CosSimShardClnt) Stop() {
	cssc.done = true
}
