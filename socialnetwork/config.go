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
	cacheMcpu      = 2000
	HTTP_ADDRS     = "http-addr"
	N_RPC_SESSIONS = 10
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

func MakeMoLSrvs(public bool) []Srv {
	return []Srv{
		Srv{"socialnetwork-mol", public, 1},
		Srv{"socialnetwork-user", public, 2},
		Srv{"socialnetwork-graph", public, 2},
	}
}

func MakeFsLibs(uname string) []*fslib.FsLib {
	dbg.DFatalf("Differentiate for different fslibs")
	fsls := make([]*fslib.FsLib, 0, N_RPC_SESSIONS)
	for i := 0; i < N_RPC_SESSIONS; i++ {
		fsl, err := fslib.MakeFsLib(proc.GetProcEnv())
		//fsl, err := fslib.MakeFsLib(sp.Tuname(uname + "-" + strconv.Itoa(i)))
		if err != nil {
			dbg.DFatalf("Error mkfsl: %v", err)
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
	return path.Join(sp.SOCIAL_NETWORK, job)
}

func MakeConfig(sc *sigmaclnt.SigmaClnt, jobname string, srvs []Srv, nsrv int, gc, public bool) (*SocialNetworkConfig, error) {
	var err error
	fsl := sc.FsLib
	fsl.MkDir(sp.SOCIAL_NETWORK, 0777)
	if err = fsl.MkDir(JobDir(jobname), 0777); err != nil {
		fmt.Printf("Mkdir %v err %v\n", JobDir(jobname), err)
		return nil, err
	}
	// Create a cache clnt.
	var cc *cachedsvcclnt.CachedSvcClnt
	var cm *cachedsvc.CacheMgr
	if nsrv > 0 {
		dbg.DPrintf(dbg.SOCIAL_NETWORK, "social network running with cached")
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
		p := proc.MakeProc(srv.Name, []string{strconv.FormatBool(srv.Public), jobname})
		p.SetMcpu(srv.Mcpu)
		if _, errs := sc.SpawnBurst([]*proc.Proc{p}, 2); len(errs) > 0 {
			dbg.DFatalf("Error burst-spawnn proc %v: %v", p, errs)
			return nil, err
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
