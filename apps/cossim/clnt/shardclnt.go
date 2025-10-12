package clnt

import (
	"fmt"
	"sync"
	"time"

	"sigmaos/apps/cossim"
	"sigmaos/apps/cossim/proto"
	"sigmaos/apps/epcache"
	epcacheclnt "sigmaos/apps/epcache/clnt"
	epcacheproto "sigmaos/apps/epcache/proto"
	db "sigmaos/debug"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

type clnt struct {
	srvID           string
	outstandingReqs uint64
	c               *CosSimClnt
}

func newClnt(srvID string, c *CosSimClnt) *clnt {
	return &clnt{
		srvID:           srvID,
		outstandingReqs: 0,
		c:               c,
	}
}

func (c *clnt) Get() *CosSimClnt {
	c.outstandingReqs++
	return c.c
}

func (c *clnt) Put() {
	c.outstandingReqs--
}

func (c *clnt) GetNOutstanding() uint64 {
	return c.outstandingReqs
}

func (c *clnt) GetSrvID() string {
	return c.srvID
}

type CosSimShardClnt struct {
	mu         sync.Mutex
	fsl        *fslib.FsLib
	clnts      map[string]*clnt
	clntsSlice []*CosSimClnt
	epcc       *epcacheclnt.EndpointCacheClnt
	lastEPV    epcache.Tversion
	done       bool
}

func NewCosSimShardClnt(fsl *fslib.FsLib, epcc *epcacheclnt.EndpointCacheClnt) (*CosSimShardClnt, error) {
	cssc := &CosSimShardClnt{
		fsl:     fsl,
		clnts:   make(map[string]*clnt),
		epcc:    epcc,
		lastEPV: epcache.NO_VERSION,
		done:    false,
	}
	go cssc.monitorShards()
	return cssc, nil
}

func (cssc *CosSimShardClnt) CosSimLeastLoaded(v []float64, ranges []*proto.VecRange) (uint64, float64, error) {
	start := time.Now()
	srvID, clnt, err := cssc.GetLeastLoadedClnt()
	if err != nil {
		db.DPrintf(db.COSSIMCLNT_ERR, "Err GetLeastLoadedClnt: %v", err)
		return 0, 0.0, err
	}
	db.DPrintf(db.COSSIMCLNT, "Least loaded server: %v lat:%v", srvID, time.Since(start))
	defer cssc.PutClnt(srvID)
	return clnt.CosSim(v, ranges)
}

func (cssc *CosSimShardClnt) GetLeastLoadedClnt() (string, *CosSimClnt, error) {
	cssc.mu.Lock()
	defer cssc.mu.Unlock()

	if len(cssc.clnts) == 0 {
		return sp.NOT_SET, nil, fmt.Errorf("No cossim clients available")
	}

	var minClnt *clnt
	for _, clnt := range cssc.clnts {
		if minClnt == nil || clnt.GetNOutstanding() < minClnt.GetNOutstanding() {
			minClnt = clnt
		}
	}
	return minClnt.GetSrvID(), minClnt.Get(), nil
}

func (cssc *CosSimShardClnt) GetClnt(srvID string) (*CosSimClnt, error) {
	cssc.mu.Lock()
	defer cssc.mu.Unlock()

	clnt, ok := cssc.clnts[srvID]
	if ok {
		return clnt.Get(), nil
	}
	c, err := NewCosSimClnt(cssc.fsl, cssc.epcc, srvID)
	if err != nil {
		return nil, err
	}
	cssc.clnts[srvID] = newClnt(srvID, c)
	return clnt.Get(), nil
}

func (cssc *CosSimShardClnt) PutClnt(srvID string) {
	cssc.mu.Lock()
	defer cssc.mu.Unlock()

	clnt, ok := cssc.clnts[srvID]
	if !ok {
		db.DPrintf(db.ERROR, "Err put clnt unknown clnt: %v", srvID)
		return
	}
	clnt.Put()
}

func (cssc *CosSimShardClnt) monitorShards() {
	for !cssc.done {
		// Get EPs
		instances, v, err := cssc.epcc.GetEndpoints(cossim.COSSIM, cssc.lastEPV)
		if err != nil {
			db.DPrintf(db.COSSIMCLNT_ERR, "Err GetEndpoints: %v", err)
			time.Sleep(1 * time.Second)
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
		cssc.clnts[i.ID] = newClnt(i.ID, clnt)
		cssc.clntsSlice = append(cssc.clntsSlice, clnt)
	}
}

func (cssc *CosSimShardClnt) Stop() {
	cssc.done = true
}
