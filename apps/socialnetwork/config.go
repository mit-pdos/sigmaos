package socialnetwork

import (
	"fmt"
	"path/filepath"

	cachegrpclnt "sigmaos/apps/cache/cachegrp/clnt"
	cachegrpmgr "sigmaos/apps/cache/cachegrp/mgr"
	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/fslib"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
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
)

type Srv struct {
	Name string
	Args []string
	Mcpu proc.Tmcpu
}

func JobHTTPAddrsPath(job string) string {
	return filepath.Join(JobDir(job), HTTP_ADDRS)
}

func GetJobHTTPAddrs(fsl *fslib.FsLib, job string) (sp.Taddrs, error) {
	mnt, err := fsl.ReadEndpoint(JobHTTPAddrsPath(job))
	if err != nil {
		return nil, err
	}
	return mnt.Addrs(), err
}

func NewFsLib(uname string, npc *dialproxyclnt.DialProxyClnt) (*fslib.FsLib, error) {
	pe := proc.GetProcEnv()
	fsl, err := sigmaclnt.NewFsLib(proc.NewAddedProcEnv(pe), npc)
	if err != nil {
		db.DPrintf(db.ERROR, "Error newfsl: %v", err)
		return nil, err
	}
	return fsl, nil
}

type SocialNetworkConfig struct {
	*sigmaclnt.SigmaClnt
	srvs      []Srv
	pids      []sp.Tpid
	CacheClnt *cachegrpclnt.CachedSvcClnt
	cacheMgr  *cachegrpmgr.CacheMgr
}

func JobDir(job string) string {
	return filepath.Join(SOCIAL_NETWORK, job)
}

func NewConfig(sc *sigmaclnt.SigmaClnt, jobname string, srvs []Srv, nsrv int, gc bool) (*SocialNetworkConfig, error) {
	var err error
	fsl := sc.FsLib
	fsl.MkDir(SOCIAL_NETWORK, 0777)
	if err = fsl.MkDir(JobDir(jobname), 0777); err != nil {
		fmt.Printf("Mkdir %v err %v\n", JobDir(jobname), err)
		return nil, err
	}
	// Create a cache clnt.
	var cc *cachegrpclnt.CachedSvcClnt
	var cm *cachegrpmgr.CacheMgr
	if nsrv > 0 {
		db.DPrintf(db.SOCIAL_NETWORK, "social network running with cached: %v caches", nsrv)
		cm, err = cachegrpmgr.NewCacheMgr(sc, jobname, nsrv, proc.Tmcpu(cacheMcpu), gc)
		if err != nil {
			db.DPrintf(db.ERROR, "Error NewCacheMgr %v", err)
			return nil, err
		}
		cc = cachegrpclnt.NewCachedSvcClnt(sc.FsLib, jobname)
	}

	// Start procs
	pids := make([]sp.Tpid, 0, len(srvs))
	for _, srv := range srvs {
		db.DPrintf(db.TEST, "Start %v", srv.Name)
		p := proc.NewProc(srv.Name, append([]string{jobname}, srv.Args...))
		p.SetMcpu(srv.Mcpu)
		if err := sc.Spawn(p); err != nil {
			db.DPrintf(db.ERROR, "Error burst-spawnn proc %v: %v", p, err)
			return nil, err
		}
		if !gc {
			p.AppendEnv("GOGC", "off")
		}
		if err = sc.WaitStart(p.GetPid()); err != nil {
			db.DPrintf(db.ERROR, "Error spawn proc %v: %v", p, err)
			return nil, err
		}
		db.DPrintf(db.TEST, "Start done %v", srv.Name)
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
