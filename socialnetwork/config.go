package socialnetwork

import (
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/cacheclnt"
	"fmt"
	"path"
	"strconv"
	dbg "sigmaos/debug"
	sp "sigmaos/sigmap"
)

const cacheNcore = 2

type Srv struct {
	Name   string
	Public bool
	Ncore  proc.Tcore
}

func MakeMoLSrvs(public bool) []Srv {
	return []Srv{
		Srv{"socialnetwork-mol", public, 1},
		Srv{"socialnetwork-user", public, 2},
	}
}

type SocialNetworkConfig struct {
	*sigmaclnt.SigmaClnt
	srvs      []Srv
	pids      []proc.Tpid
	CacheClnt *cacheclnt.CacheClnt
	cacheMgr  *cacheclnt.CacheMgr
}

func JobDir(job string) string {
	return path.Join(sp.SOCIAL_NETWORK, job)
}

func MakeConfig(sc *sigmaclnt.SigmaClnt, jobname string, srvs []Srv, nshard int, public bool) (*SocialNetworkConfig, error) {
	var err error
	fsl := sc.FsLib
	fsl.MkDir(sp.SOCIAL_NETWORK, 0777)
	if err = fsl.MkDir(JobDir(jobname), 0777); err != nil {
		fmt.Printf("Mkdir %v err %v\n", JobDir(jobname), err)
		return nil, err
	}
	// Create a cache clnt.
	var cc *cacheclnt.CacheClnt
	var cm *cacheclnt.CacheMgr
	if nshard > 0 {
		dbg.DPrintf(dbg.ALWAYS, "social network running with cached")
		cm, err = cacheclnt.MkCacheMgr(sc, jobname, nshard, proc.Tcore(cacheNcore), public)
		if err != nil {
			dbg.DFatalf("Error MkCacheMgr %v", err)
			return nil, err
		}
		cc, err = cacheclnt.MkCacheClnt(sc.FsLib, jobname)
		if err != nil {
			dbg.DFatalf("Error cacheclnt %v", err)
			return nil, err
		}
	}

	// Start procs
	pids := make([]proc.Tpid, 0, len(srvs))
	for _, srv := range srvs {
		p := proc.MakeProc(srv.Name, []string{strconv.FormatBool(srv.Public), jobname})
		p.SetNcore(srv.Ncore)
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

