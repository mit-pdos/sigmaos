package cossim

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
	"strconv"
	"sync"
	"time"

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
	"sigmaos/proxy/wasm/rpc/wasmer"
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
	bootScript       []byte
	bootScriptInput  []byte
	stopEPCJ         bool
	stopCaches       bool
}

func NewCosSimJob(sc *sigmaclnt.SigmaClnt, epcj *epsrv.EPCacheJob, cm *cachegrpmgr.CacheMgr, cc *cachegrpclnt.CachedSvcClnt, job string, nvec int, vecDim int, eagerInit bool, srvMcpu proc.Tmcpu, ncache int, cacheMcpu proc.Tmcpu, cacheGC bool, delegateInitRPCs bool) (*CosSimJob, error) {
	stopEPCJ := false
	stopCaches := false
	var err error
	// If not supplied, create epcache job
	if epcj == nil {
		stopEPCJ = true
		// Create epcache job
		epcj, err = epsrv.NewEPCacheJob(sc)
		if err != nil {
			db.DPrintf(db.COSSIMSRV_ERR, "Err epcache: %v", err)
			return nil, err
		}
	}
	// If not supplied, create cache manager
	if cm == nil {
		stopCaches = true
		// Start the cachegrp job
		cm, err = cachegrpmgr.NewCacheMgr(sc, job, ncache, cacheMcpu, cacheGC)
		if err != nil {
			db.DPrintf(db.COSSIMSRV_ERR, "Err newCacheMgr: %v", err)
			return nil, err
		}
	}
	// If not supplied, create cache client
	if cc == nil {
		cc = cachegrpclnt.NewCachedSvcClnt(sc.FsLib, job)
	}
	return newCosSimJob(sc, epcj, cm, cc, job, nvec, vecDim, eagerInit, srvMcpu, ncache, cacheMcpu, cacheGC, delegateInitRPCs, stopEPCJ, stopCaches)
}

func newCosSimJob(sc *sigmaclnt.SigmaClnt, epcj *epsrv.EPCacheJob, cm *cachegrpmgr.CacheMgr, cc *cachegrpclnt.CachedSvcClnt, job string, nvec int, vecDim int, eagerInit bool, srvMcpu proc.Tmcpu, ncache int, cacheMcpu proc.Tmcpu, cacheGC bool, delegateInitRPCs bool, stopEPCJ bool, stopCaches bool) (*CosSimJob, error) {
	// Init fs
	if err := initFS(sc, job); err != nil {
		db.DPrintf(db.COSSIMSRV_ERR, "Err initfs: %v", err)
		return nil, err
	}
	epcsrvEP, err := epcj.GetSrvEP()
	if err != nil {
		db.DPrintf(db.COSSIMSRV_ERR, "Err getSrvEP: %v", err)
		return nil, err
	}
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
	bootScript, err := wasmer.ReadBootScript(sc, "cossim_boot")
	if err != nil {
		db.DPrintf(db.ERROR, "Err read WASM boot script: %v", err)
		return nil, err
	}
	// Write the input arguments to the boot script
	inputBuf := bytes.NewBuffer(make([]byte, 0, 12))
	if err := binary.Write(inputBuf, binary.LittleEndian, uint32(ncache)); err != nil {
		return nil, err
	}
	if err := binary.Write(inputBuf, binary.LittleEndian, uint64(nvec)); err != nil {
		return nil, err
	}
	bootScriptInput := inputBuf.Bytes()
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
		bootScript:       bootScript,
		bootScriptInput:  bootScriptInput,
		stopEPCJ:         stopEPCJ,
		stopCaches:       stopCaches,
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
	p.SetBootScript(j.bootScript, j.bootScriptInput)
	p.SetRunBootScript(j.delegateInitRPCs)
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
	if j.stopCaches {
		j.cacheMgr.Stop()
	}
	if j.stopEPCJ {
		j.EPCacheJob.Stop()
	}
	return nil
}
