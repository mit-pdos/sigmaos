package hotel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strconv"
	"sync"

	"sigmaos/api/fs"
	"sigmaos/apps/cache"
	cachegrpclnt "sigmaos/apps/cache/cachegrp/clnt"
	cossimclnt "sigmaos/apps/cossim/clnt"
	cossimproto "sigmaos/apps/cossim/proto"
	epcacheclnt "sigmaos/apps/epcache/clnt"
	"sigmaos/apps/hotel/proto"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmasrv"
	"sigmaos/util/perf"
	"sigmaos/util/tracing"
)

type Match struct {
	mu         sync.Mutex
	inputVecs  map[uint64][]float64
	cossimClnt *cossimclnt.CosSimShardClnt
	cc         *cachegrpclnt.CachedSvcClnt
	pds        *sigmasrv.SigmaSrv
	tracer     *tracing.Tracer
	pHit       *perf.Perf
	pMiss      *perf.Perf
}

// Run starts the server
func RunMatchSrv(job string) error {
	s := &Match{
		inputVecs: make(map[uint64][]float64),
	}
	ssrv, err := sigmasrv.NewSigmaSrv(HOTELMATCH, s, proc.GetProcEnv())
	if err != nil {
		return err
	}
	fsl, err := NewFsLib(HOTELMATCH, ssrv.MemFs.SigmaClnt().GetDialProxyClnt())
	if err != nil {
		return err
	}

	epcc, err := epcacheclnt.NewEndpointCacheClnt(fsl)
	if err != nil {
		return err
	}
	cossimClnt, err := cossimclnt.NewCosSimShardClnt(fsl, epcc)
	if err != nil {
		return err
	}
	s.cossimClnt = cossimClnt

	s.cc = cachegrpclnt.NewCachedSvcClntEPCache(fsl, epcc, job)

	p, err := perf.NewPerf(ssrv.MemFs.SigmaClnt().ProcEnv(), perf.HOTEL_MATCH)
	if err != nil {
		db.DFatalf("NewPerf err %v\n", err)
	}
	defer p.Done()

	pMiss, err := perf.NewPerfMulti(ssrv.MemFs.SigmaClnt().ProcEnv(), perf.HOTEL_MATCH, "miss")
	if err != nil {
		db.DFatalf("NewPerf err %v\n", err)
	}
	defer pMiss.Done()
	s.pMiss = pMiss

	pHit, err := perf.NewPerfMulti(ssrv.MemFs.SigmaClnt().ProcEnv(), perf.HOTEL_MATCH, "hit")
	if err != nil {
		db.DFatalf("NewPerf err %v\n", err)
	}
	defer pHit.Done()
	s.pHit = pHit

	return ssrv.RunServer()
}

func (s *Match) getInputVec(userVecID uint64) ([]float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if vec, ok := s.inputVecs[userVecID]; ok {
		return vec, nil
	}

	b, err := s.cc.GetBytes(strconv.FormatUint(userVecID, 10))
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(b)
	vecVals := make([]float64, len(b)/8)
	for i := range vecVals {
		// Write the value to the buffer in LittleEndian mode
		if err := binary.Read(buf, binary.LittleEndian, &vecVals[i]); err != nil {
			db.DPrintf(db.ERROR, "binary.Read err: %v", err)
			return nil, err
		}
	}
	s.inputVecs[userVecID] = vecVals
	return vecVals, nil
}

// Nearby returns ids of nearby hotels order by results of ratesrv
func (s *Match) UserPreference(ctx fs.CtxI, req proto.MatchReq, res *proto.MatchRep) error {
	db.DPrintf(db.HOTEL_MATCH, "Match UserPreference: %v", req)
	cacheKey := fmt.Sprintf("user-preference-%v", req.UserID)
	if req.TryCache {
		v := &cossimproto.CosSimRep{}
		err := s.cc.Get(cacheKey, v)
		if err != nil {
			// Migration- and true-misses both count as a miss
			if cache.IsMiss(err) || cache.IsMigrating(err) {
				s.pMiss.TptTick(1.0)
				if cache.IsMigrating(err) {
					db.DPrintf(db.HOTEL_MATCH_ERR, "Try to get %v during migration", cacheKey)
				}
			} else {
				db.DPrintf(db.HOTEL_MATCH_ERR, "Err cache get: %v", err)
				return err
			}
		} else {
			s.pHit.TptTick(1.0)
			res.ID = v.ID
			res.Val = v.Val
			res.WasCached = true
			return nil
		}
	}
	inputVec, err := s.getInputVec(req.UserVecID)
	if err != nil {
		db.DPrintf(db.HOTEL_MATCH_ERR, "Err get input vec: %v", err)
		return err
	}
	id, val, err := s.cossimClnt.CosSimLeastLoaded(inputVec, req.VecRanges, true)
	if err != nil {
		db.DPrintf(db.HOTEL_MATCH_ERR, "Err CosSimLeastLoaded: %v", err)
		return err
	}
	// Cache must have missed. Insert result in the cache.
	if req.TryCache {
		err := s.cc.Put(cacheKey, &cossimproto.CosSimRep{
			ID:  id,
			Val: val,
		})
		if err != nil {
			// If put fails due to migration, move on
			if cache.IsMigrating(err) {
				db.DPrintf(db.HOTEL_MATCH_ERR, "Try to put %v during migration", cacheKey)
				return nil
			}
			db.DPrintf(db.HOTEL_MATCH_ERR, "Err CachePut: %v", err)
			return err
		}
	}
	db.DPrintf(db.HOTEL_MATCH, "Match done: %v %v", req, res)
	return nil
}
