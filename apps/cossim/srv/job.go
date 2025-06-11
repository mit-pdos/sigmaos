package cossim

import (
	"path/filepath"
	"strconv"
	"sync"
	"time"

	cachegrpclnt "sigmaos/apps/cache/cachegrp/clnt"
	cachegrpmgr "sigmaos/apps/cache/cachegrp/mgr"
	"sigmaos/apps/cossim"
	"sigmaos/apps/cossim/clnt"
	"sigmaos/apps/cossim/proto"
	"sigmaos/apps/epcache"
	epsrv "sigmaos/apps/epcache/srv"
	db "sigmaos/debug"
	"sigmaos/proc"
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

// Write vector DB to cache srv
func writeVectorsToCache(cc *cachegrpclnt.CachedSvcClnt, vecs []*proto.Vector) error {
	for id, v := range vecs {
		if err := cc.Put(strconv.Itoa(id), v); err != nil {
			return err
		}
	}
	return nil
}

type CosSimJob struct {
	mu  sync.Mutex
	job string
	*sigmaclnt.SigmaClnt
	EPCacheJob *epsrv.EPCacheJob
	epcsrvEP   *sp.Tendpoint
	cacheClnt  *cachegrpclnt.CachedSvcClnt
	cacheMgr   *cachegrpmgr.CacheMgr
	cachePN    string
	nvec       int
	vecDim     int
	eagerInit  bool
	vecs       []*proto.Vector
	srvMcpu    proc.Tmcpu
	srvs       []*proc.Proc
	cscs       *clnt.CosSimShardClnt
}

func NewCosSimJob(sc *sigmaclnt.SigmaClnt, job string, nvec int, vecDim int, eagerInit bool, srvMcpu proc.Tmcpu, ncache int, cacheMcpu proc.Tmcpu, cacheGC bool) (*CosSimJob, error) {
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
	if err := writeVectorsToCache(cc, vecs); err != nil {
		db.DPrintf(db.COSSIMSRV_ERR, "Err writeVectors: %v", err)
		return nil, err
	}
	cscs, err := clnt.NewCosSimShardClnt(sc.FsLib, epcj.Clnt)
	if err != nil {
		db.DPrintf(db.COSSIMSRV_ERR, "Err newCosSimShardClnt: %v", err)
		return nil, err
	}
	return &CosSimJob{
		job:        job,
		SigmaClnt:  sc,
		EPCacheJob: epcj,
		epcsrvEP:   epcsrvEP,
		cacheClnt:  cc,
		cachePN:    cc.Server(0),
		cacheMgr:   cm,
		nvec:       nvec,
		vecDim:     vecDim,
		vecs:       vecs,
		eagerInit:  eagerInit,
		srvMcpu:    srvMcpu,
		srvs:       []*proc.Proc{},
		cscs:       cscs,
	}, nil
}

func (j *CosSimJob) GetClnt(srvID string) (*clnt.CosSimClnt, error) {
	return j.cscs.GetClnt(srvID)
}

// Add a new cossim server
func (j *CosSimJob) AddSrv() (*proc.Proc, time.Duration, error) {
	p := proc.NewProc("cossim-srv-cpp", []string{j.cachePN, strconv.Itoa(j.nvec), strconv.Itoa(j.vecDim), strconv.FormatBool(j.eagerInit)})
	p.GetProcEnv().UseSPProxy = true
	p.SetMcpu(j.srvMcpu)
	p.SetCachedEndpoint(epcache.EPCACHE, j.epcsrvEP)
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
	return p, startLatency, nil
}

func (j *CosSimJob) Stop() error {
	j.cscs.Stop()
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
