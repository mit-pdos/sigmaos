package socialnetwork

import (
	"fmt"
	"path"

	"sigmaos/cachedsvc"
	"sigmaos/cachedsvcclnt"
	dbg "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"strconv"
)

const (
	SOCIAL_NETWORK          = sp.NAMED + "socialnetwork/"
	SOCIAL_NETWORK_USER     = SOCIAL_NETWORK + "user"
	SOCIAL_NETWORK_GRAPH    = SOCIAL_NETWORK + "graph"
	SOCIAL_NETWORK_POST     = SOCIAL_NETWORK + "post"
	SOCIAL_NETWORK_TIMELINE = SOCIAL_NETWORK + "timeline"
	SOCIAL_NETWORK_HOME     = SOCIAL_NETWORK + "home"
	SOCIAL_NETWORK_URL      = SOCIAL_NETWORK + "url"
	SOCIAL_NETWORK_TEXT     = SOCIAL_NETWORK + "text"
	SOCIAL_NETWORK_COMPOSE  = SOCIAL_NETWORK + "compose"
	SOCIAL_NETWORK_MEDIA    = SOCIAL_NETWORK + "media"
	cacheMcpu               = 1000
	HTTP_ADDRS              = "http-addr"
	N_RPC_SESSIONS          = 10
)

type Srv struct {
	Name   string
	Public bool
	Mcpu   proc.Tmcpu
}

func JobHTTPAddrsPath(job string) string {
	return path.Join(JobDir(job), HTTP_ADDRS)
}

func GetJobHTTPAddrs(fsl *fslib.FsLib, job string) (sp.Taddrs, error) {
	mnt, err := fsl.ReadMount(JobHTTPAddrsPath(job))
	if err != nil {
		return nil, err
	}
	return mnt.Addr, err
}

func NewFsLibs(uname string) []*fslib.FsLib {
	fsls := make([]*fslib.FsLib, 0, N_RPC_SESSIONS)
	for i := 0; i < N_RPC_SESSIONS; i++ {
		pe := proc.GetProcEnv()
		fsl, err := fslib.NewFsLib(proc.NewAddedProcEnv(pe, i))
		if err != nil {
			dbg.DFatalf("Error newfsl: %v", err)
		}
		fsls = append(fsls, fsl)
	}
	return fsls
}

type SocialNetworkConfig struct {
	*sigmaclnt.SigmaClnt
	srvs      []Srv
	pids      []sp.Tpid
	CacheClnt *cachedsvcclnt.CachedSvcClnt
	cacheMgr  *cachedsvc.CacheMgr
}

func JobDir(job string) string {
	return path.Join(SOCIAL_NETWORK, job)
}

func NewConfig(sc *sigmaclnt.SigmaClnt, jobname string, srvs []Srv, nsrv int, gc, public bool) (*SocialNetworkConfig, error) {
	var err error
	fsl := sc.FsLib
	fsl.MkDir(SOCIAL_NETWORK, 0777)
	if err = fsl.MkDir(JobDir(jobname), 0777); err != nil {
		fmt.Printf("Mkdir %v err %v\n", JobDir(jobname), err)
		return nil, err
	}
	// Create a cache clnt.
	var cc *cachedsvcclnt.CachedSvcClnt
	var cm *cachedsvc.CacheMgr
	if nsrv > 0 {
		dbg.DPrintf(dbg.SOCIAL_NETWORK, "social network running with cached: %v caches", nsrv)
		cm, err = cachedsvc.MkCacheMgr(sc, jobname, nsrv, proc.Tmcpu(cacheMcpu), gc, public)
		if err != nil {
			dbg.DFatalf("Error MkCacheMgr %v", err)
			return nil, err
		}
		cc, err = cachedsvcclnt.MkCachedSvcClnt([]*fslib.FsLib{sc.FsLib}, jobname)
		if err != nil {
			dbg.DFatalf("Error cacheclnt %v", err)
			return nil, err
		}
	}

	// Start procs
	pids := make([]sp.Tpid, 0, len(srvs))
	for _, srv := range srvs {
		p := proc.NewProc(srv.Name, []string{strconv.FormatBool(srv.Public), jobname})
		p.SetMcpu(srv.Mcpu)
		if _, errs := sc.SpawnBurst([]*proc.Proc{p}, 2); len(errs) > 0 {
			dbg.DFatalf("Error burst-spawnn proc %v: %v", p, errs)
			return nil, err
		}
		if !gc {
			p.AppendEnv("GOGC", "off")
		}
		if err = sc.WaitStart(p.GetPid()); err != nil {
			dbg.DFatalf("Error spawn proc %v: %v", p, err)
			return nil, err
		}
		pids = append(pids, p.GetPid())
	}
	return &SocialNetworkConfig{sc, srvs, pids, cc, cm}, nil
}

func (snCfg *SocialNetworkConfig) Stop() error {
	for _, pid := range snCfg.pids {
		if err := snCfg.Evict(pid); err != nil {
			return err
		}
		if _, err := snCfg.WaitExit(pid); err != nil {
			return err
		}
	}
	if snCfg.cacheMgr != nil {
		snCfg.cacheMgr.Stop()
	}
	return nil
}
