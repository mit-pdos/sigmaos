package socialnetwork

import (
	"sigmaos/proc"
	"sigmaos/sigmap"
	"sigmaos/kernel"
	"sigmaos/bootkernelclnt"
	"sigmaos/container"
	"sigmaos/sigmaclnt"
	"sigmaos/rand"
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

type SigmaConfig struct {
	*sigmaclnt.SigmaClnt
	kclnts  []*bootkernelclnt.Kernel
	killidx int
}

// Inspired by test.makeSysClnt in test/test.go
func MakeSigmaConfig(start, overlays bool, tag, rootNamedIP string) (*SigmaConfig, error) {
	namedport := sigmap.MkTaddrs([]string{NAMEDPORT})
	kernelid := ""
	// configure IP
	var containerIP string
	var err error
	if rootNamedIP == "" {
		containerIP, err = container.LocalIP()
		if err != nil {
			return nil, err
		}
	} else {
		containerIP = rootNamedIP
	}
	// start kernel
	if start {
		kernelid = bootkernelclnt.GenKernelId()
		ip, err := bootkernelclnt.Start(kernelid, tag, BOOT_ALL, namedport, overlays)
		if err != nil {
			return nil, err
		}
		containerIP = ip
	}
	// retrieve kernel clients
	proc.SetPid(proc.Tpid("socialnetowrk-" + proc.GenPid().String()))
	namedAddr, err := kernel.SetNamedIP(containerIP, namedport)
	if err != nil {
		fmt.Printf("Failed to set kernel IP!")
		return nil, err
	}
	k, err := bootkernelclnt.MkKernelClnt(kernelid, "socialnetwork", containerIP, namedAddr)
	if err != nil {
		fmt.Printf("Failed to make kernel client!")
		return nil, err
	}
	return &SigmaConfig {
		SigmaClnt: k.SigmaClnt,
		kclnts:    []*bootkernelclnt.Kernel{k},
		killidx:   0,
	}, nil
}

func (sigmaCfg *SigmaConfig) Shutdown() error {
	for i := len(sigmaCfg.kclnts) - 1; i >= 0; i-- {
		if err := sigmaCfg.kclnts[i].Shutdown(); err != nil {
			return err
		}
	}
	return nil
}

type SocialNetworkConfig struct {
	*SigmaConfig
	srvs  []Srv
	pids  []proc.Tpid
}

func JobDir(job string) string {
	return path.Join(SOCIAL_NETWORK_ROOT, job)
}

func MakeSocialNetworkConfig(sigmaCfg *SigmaConfig, jobname string, srvs []Srv) (*SocialNetworkConfig, error) {
	fsl := sigmaCfg.FsLib
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
		if _, errs := sigmaCfg.SpawnBurst([]*proc.Proc{p}, 2); len(errs) > 0 {
			dbg.DFatalf("Error burst-spawnn proc %v: %v", p, errs)
			return nil, err
		}
		if err = sigmaCfg.WaitStart(p.GetPid()); err != nil {
			dbg.DFatalf("Error spawn proc %v: %v", p, err)
			return nil, err
		}
		pids = append(pids, p.GetPid())
	}
	return &SocialNetworkConfig{sigmaCfg, srvs, pids}, nil
}

func MakeDefaultSocialNetworkConfig() (*SocialNetworkConfig, error) {
	overlays := false
	sigmaCfg, err := MakeSigmaConfig(true, overlays, "", "")
	if err != nil {
		return nil, err
	}
	srvs := MakeMoLSrvs(overlays)
	return MakeSocialNetworkConfig(sigmaCfg, rand.String(8), srvs)
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

