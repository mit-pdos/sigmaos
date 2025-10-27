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
	rpcproto "sigmaos/rpc/proto"
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
	mu      sync.Mutex
	fsl     *fslib.FsLib
	clnts   map[string]*clnt
	epcc    *epcacheclnt.EndpointCacheClnt
	lastEPV epcache.Tversion
	done    bool
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

func (cssc *CosSimShardClnt) CosSimLeastLoaded(v []float64, ranges []*proto.VecRange, retry bool) (uint64, float64, error) {
	ignore := map[string]bool{}
	// Keep retrying, ignoring clients which produced errors before
	for {
		start := time.Now()
		srvID, clnt, err := cssc.getLeastLoadedClnt(ignore)
		if err != nil {
			db.DPrintf(db.COSSIMCLNT_ERR, "Err GetLeastLoadedClnt: %v", err)
			return 0, 0.0, err
		}
		db.DPrintf(db.COSSIMCLNT, "Least loaded server: %v lat:%v", srvID, time.Since(start))
		id, val, err := cssc.runReq(srvID, clnt, v, ranges)
		// Optionally retry
		if err != nil && retry {
			ignore[srvID] = true
			db.DPrintf(db.COSSIMCLNT_ERR, "Err runReq[%v] retry: %v", srvID, err)
			continue
		}
		return id, val, err
	}
}

func (cssc *CosSimShardClnt) runReq(srvID string, clnt *CosSimClnt, v []float64, ranges []*proto.VecRange) (uint64, float64, error) {
	defer cssc.PutClnt(srvID)
	return clnt.CosSim(v, ranges)
}

func (cssc *CosSimShardClnt) GetLeastLoadedClnt() (string, *CosSimClnt, error) {
	return cssc.getLeastLoadedClnt(nil)
}

func (cssc *CosSimShardClnt) getLeastLoadedClnt(ignore map[string]bool) (string, *CosSimClnt, error) {
	cssc.mu.Lock()
	defer cssc.mu.Unlock()

	if len(cssc.clnts) == 0 {
		return sp.NOT_SET, nil, fmt.Errorf("No cossim clients available, ignoring %v", ignore)
	}

	var minClnt *clnt
	for _, clnt := range cssc.clnts {
		if ignore[clnt.GetSrvID()] {
			continue
		}
		if minClnt == nil || clnt.GetNOutstanding() < minClnt.GetNOutstanding() {
			minClnt = clnt
		}
	}
	if minClnt == nil {
		return sp.NOT_SET, nil, fmt.Errorf("No cossim clients available, ignoring %v", ignore)
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
		cssc.updateClnts(instances)
		// Update last endpoint version
		cssc.lastEPV = v
	}
}

func (cssc *CosSimShardClnt) updateClnts(instances []*epcacheproto.Instance) {
	cssc.mu.Lock()
	defer cssc.mu.Unlock()

	// Note servers which are still registered as being up
	stillUp := make(map[string]bool)
	for _, i := range instances {
		// Server is still up
		stillUp[i.ID] = true
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
	}
	for id, _ := range cssc.clnts {
		if !stillUp[id] {
			// Server is down
			db.DPrintf(db.COSSIMCLNT, "Server %v no longer up, removing clnt", id)
			delete(cssc.clnts, id)
		}
	}
}

func (cssc *CosSimShardClnt) GetAllServerMetrics() (map[string]*rpcproto.MetricsRep, error) {
	cssc.mu.Lock()
	clnts := make(map[string]*clnt)
	for srvID, clnt := range cssc.clnts {
		clnts[srvID] = clnt
	}
	cssc.mu.Unlock()

	metrics := make(map[string]*rpcproto.MetricsRep)
	for srvID, clnt := range clnts {
		rep, err := clnt.c.GetMetrics()
		if err != nil {
			db.DPrintf(db.COSSIMCLNT_ERR, "Err GetMetrics for srv %v: %v", srvID, err)
			continue
		}
		metrics[srvID] = rep
	}
	return metrics, nil
}

func (cssc *CosSimShardClnt) Stop() {
	cssc.done = true
}
