package socialnetwork

import (
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"fmt"
	"path"
	"strconv"
	dbg "sigmaos/debug"
)

const (
	BOOT_REALM = "realm"
	BOOT_ALL   = "all"
	BOOT_NAMED = "named"
	BOOT_NODE  = "node"
	NAMEDPORT  = ":1111"

	SOCIAL_NETWORK_ROOT = "name/socialnetwork/" 
)

type Srv struct {
	Name   string
	Public bool
	Ncore  proc.Tcore
}

func MakeMoLSrvs(public bool) []Srv {
	return []Srv{
		Srv{"socialnetwork-mol", public, 1},
	}
}

type SocialNetworkConfig struct {
	*sigmaclnt.SigmaClnt
	srvs  []Srv
	pids  []proc.Tpid
}

func JobDir(job string) string {
	return path.Join(SOCIAL_NETWORK_ROOT, job)
}

func MakeConfig(sc *sigmaclnt.SigmaClnt, jobname string, srvs []Srv) (*SocialNetworkConfig, error) {
	fsl := sc.FsLib
	fsl.MkDir(SOCIAL_NETWORK_ROOT, 0777)
	if err := fsl.MkDir(JobDir(jobname), 0777); err != nil {
		fmt.Printf("Mkdir %v err %v\n", JobDir(jobname), err)
		return nil, err
	}

	var err error
	pids := make([]proc.Tpid, 0, len(srvs))
	for _, srv := range srvs {
		p := proc.MakeProc(srv.Name, []string{strconv.FormatBool(srv.Public)})
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
	return &SocialNetworkConfig{sc, srvs, pids}, nil
}

func (molCfg *SocialNetworkConfig) Stop() error {
	for _, pid := range molCfg.pids {
		if err := molCfg.Evict(pid); err != nil {
			return err
		}
		if _, err := molCfg.WaitExit(pid); err != nil {
			return err
		}
	}
	return nil
}

