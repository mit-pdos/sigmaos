package cossim

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"sigmaos/apps/cache"
	cachegrpclnt "sigmaos/apps/cache/cachegrp/clnt"
	cachegrpmgr "sigmaos/apps/cache/cachegrp/mgr"
	cacheproto "sigmaos/apps/cache/proto"
	"sigmaos/apps/cossim"
	"sigmaos/apps/cossim/clnt"
	cossimproto "sigmaos/apps/cossim/proto"
	"sigmaos/apps/epcache"
	epsrv "sigmaos/apps/epcache/srv"
	db "sigmaos/debug"
	"sigmaos/proc"
	rpcclnt "sigmaos/rpc/clnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

func JobDir(jobname string) string {
	return filepath.Join(cossim.COSSIM, jobname)
}

func initFS(sc *sigmaclnt.SigmaClnt, jobname string) error {
	if err := sc.MkDir(cossim.COSSIM, 0777); err != nil {
		db.DPrintf(db.ERROR, "Mkdir %v err %v", JobDir(jobname), err)
		return err
	}
	if err := sc.MkDir(JobDir(jobname), 0777); err != nil {
		db.DPrintf(db.ERROR, "Mkdir %v err %v", JobDir(jobname), err)
		return err
	}
	return nil
}

func marshalVec(v *cossimproto.Vector) ([]byte, error) {
	b := make([]byte, 0)
	buf := bytes.NewBuffer(b)
	buf.Grow(8 * len(v.Vals))
	for _, val := range v.Vals {
		// Write the value to the buffer in LittleEndian mode
		if err := binary.Write(buf, binary.LittleEndian, val); err != nil {
			db.DPrintf(db.ERROR, "binary.Write value %v: err %v", val, err)
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// Write vector DB to cache srv
func writeVectorsToCache(cc *cachegrpclnt.CachedSvcClnt, vecs []*cossimproto.Vector) ([]string, error) {
	vecKeys := make([]string, len(vecs))
	m := make(map[string][]byte)
	for i, v := range vecs {
		id := strconv.Itoa(i)
		vecKeys[i] = id
		b, err := marshalVec(v)
		if err != nil {
			return nil, err
		}
		if err := cc.PutBytes(id, b); err != nil {
			return nil, err
		}
		m[id] = b
	}
	shard := &cacheproto.ShardReq{
		Vals: m,
	}
	db.DPrintf(db.TEST, "Put all vecs")
	// Also store all values in a single key
	if err := cc.Put("all-vecs", shard); err != nil {
		db.DPrintf(db.ERROR, "Error write all vecs")
		return nil, err
	}
	db.DPrintf(db.TEST, "Done Put all vecs")
	return vecKeys, nil
}

type CosSimJob struct {
	mu  sync.Mutex
	job string
	*sigmaclnt.SigmaClnt
	EPCacheJob       *epsrv.EPCacheJob
	epcsrvEP         *sp.Tendpoint
	cacheClnt        *cachegrpclnt.CachedSvcClnt
	cacheMgr         *cachegrpmgr.CacheMgr
	cachePNBase      string
	cacheEPs         map[string]*sp.Tendpoint
	ncache           int
	nvec             int
	vecDim           int
	eagerInit        bool
	vecs             []*cossimproto.Vector
	vecKeys          []string
	srvMcpu          proc.Tmcpu
	srvs             []*proc.Proc
	Clnt             *clnt.CosSimShardClnt
	delegateInitRPCs bool
}

func NewCosSimJob(sc *sigmaclnt.SigmaClnt, job string, nvec int, vecDim int, eagerInit bool, srvMcpu proc.Tmcpu, ncache int, cacheMcpu proc.Tmcpu, cacheGC bool, delegateInitRPCs bool) (*CosSimJob, error) {
	// Init fs
	if err := initFS(sc, job); err != nil {
		db.DPrintf(db.COSSIMSRV_ERR, "Err initfs: %v", err)
		return nil, err
	}
	// Create epcache job
	epcj, err := epsrv.NewEPCacheJob(sc)
	if err != nil {
		db.DPrintf(db.COSSIMSRV_ERR, "Err epcache: %v", err)
		return nil, err
	}
	epcsrvEP, err := epcj.GetSrvEP()
	if err != nil {
		db.DPrintf(db.COSSIMSRV_ERR, "Err getSrvEP: %v", err)
		return nil, err
	}
	// Start the cachegrp job
	cm, err := cachegrpmgr.NewCacheMgr(sc, job, ncache, cacheMcpu, cacheGC)
	if err != nil {
		db.DPrintf(db.COSSIMSRV_ERR, "Err newCacheMgr: %v", err)
		return nil, err
	}
	cc := cachegrpclnt.NewCachedSvcClnt(sc.FsLib, job)
	vecs := cossim.NewVectors(nvec, vecDim)
	vecKeys, err := writeVectorsToCache(cc, vecs)
	if err != nil {
		db.DPrintf(db.COSSIMSRV_ERR, "Err writeVectors: %v", err)
		return nil, err
	}
	cacheEPs, err := cc.GetEndpoints()
	if err != nil {
		db.DPrintf(db.COSSIMSRV_ERR, "Err get cache EPs: %v", err)
		return nil, err
	}
	cscs, err := clnt.NewCosSimShardClnt(sc.FsLib, epcj.Clnt)
	if err != nil {
		db.DPrintf(db.COSSIMSRV_ERR, "Err newCosSimShardClnt: %v", err)
		return nil, err
	}
	return &CosSimJob{
		job:              job,
		SigmaClnt:        sc,
		EPCacheJob:       epcj,
		epcsrvEP:         epcsrvEP,
		cacheClnt:        cc,
		cachePNBase:      filepath.Dir(cc.Server(0)),
		cacheEPs:         cacheEPs,
		cacheMgr:         cm,
		ncache:           ncache,
		nvec:             nvec,
		vecDim:           vecDim,
		vecs:             vecs,
		vecKeys:          vecKeys,
		eagerInit:        eagerInit,
		srvMcpu:          srvMcpu,
		srvs:             []*proc.Proc{},
		Clnt:             cscs,
		delegateInitRPCs: delegateInitRPCs,
	}, nil
}

func (j *CosSimJob) GetClnt(srvID string) (*clnt.CosSimClnt, error) {
	return j.Clnt.GetClnt(srvID)
}

func (j *CosSimJob) AddSrvWithSigmaPath(pn string) (*proc.Proc, time.Duration, error) {
	return j.addSrv(pn)
}

// Add a new cossim server
func (j *CosSimJob) AddSrv() (*proc.Proc, time.Duration, error) {
	return j.addSrv(sp.NOT_SET)
}

func (j *CosSimJob) addSrv(sigmaPath string) (*proc.Proc, time.Duration, error) {
	p := proc.NewProc("cossim-srv-cpp", []string{j.cachePNBase, strconv.Itoa(j.ncache), strconv.Itoa(j.nvec), strconv.Itoa(j.vecDim), strconv.FormatBool(j.eagerInit)})
	if sigmaPath != sp.NOT_SET {
		p.PrependSigmaPath(sigmaPath)
	}
	p.GetProcEnv().UseSPProxy = true
	p.GetProcEnv().UseSPProxyProcClnt = true
	p.SetMcpu(j.srvMcpu)
	p.SetCachedEndpoint(epcache.EPCACHE, j.epcsrvEP)
	for pn, ep := range j.cacheEPs {
		p.SetCachedEndpoint(pn, ep)
	}
	cacheGetReqs := cachegrpclnt.NewMultiGetReqs(j.vecKeys, j.ncache, cache.NSHARD)
	nreqs := 0
	for i := 0; i < len(cacheGetReqs); i++ {
		server := i
		getReq := cacheGetReqs[server]
		cachesrvPN := j.cacheClnt.Server(server)
		db.DPrintf(db.TEST, "MultiGetReq for new cachesrv: %v -> %v", cachesrvPN, len(getReq.Gets))
		iniov, err := rpcclnt.WrapRPCRequest("CacheSrv.MultiGet", getReq)
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error wrap & marshal getReq: %v", err)
			return nil, 0, err
		}
		p.AddInitializationRPC(cachesrvPN, iniov, 3)
		totalInIOVLen := 0
		for _, b := range iniov {
			totalInIOVLen += len(b)
		}
		db.DPrintf(db.TEST, "Delegated RPC(%v) total len: %v", nreqs, totalInIOVLen)
		nreqs++
	}
	// Ask for spproxy to run delegated initialization RPCs on behalf of the proc
	p.SetDelegateInit(j.delegateInitRPCs)
	start := time.Now()
	if err := j.Spawn(p); err != nil {
		return nil, 0, err
	}
	if err := j.WaitStart(p.GetPid()); err != nil {
		return nil, 0, err
	}
	startLatency := time.Since(start)

	j.mu.Lock()
	defer j.mu.Unlock()

	j.srvs = append(j.srvs, p)
	if len(j.srvs) == 1 {
		time.Sleep(time.Second)
	}
	return p, startLatency, nil
}

func (j *CosSimJob) Stop() error {
	j.Clnt.Stop()
	for _, p := range j.srvs {
		db.DPrintf(db.TEST, "Evict cossim %v", p.GetPid())
		if err := j.Evict(p.GetPid()); err != nil {
			db.DPrintf(db.ERROR, "Err evict: %v", err)
			return err
		}
		status, err := j.WaitExit(p.GetPid())
		if err != nil {
			db.DPrintf(db.ERROR, "Err WaitExit: %v", err)
			return err
		}
		db.DPrintf(db.TEST, "CPP proc exited, status: %v", status)
		if !status.IsStatusEvicted() {
			db.DPrintf(db.ERROR, "Proc wrong exit status: %v", status)
			return err
		}
	}
	j.cacheMgr.Stop()
	j.EPCacheJob.Stop()
	return nil
}
