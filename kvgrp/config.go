package kvgrp

import (
	"fmt"

	"sigmaos/cachesrv"
	db "sigmaos/debug"
	"sigmaos/replraft"
	"sigmaos/replsrv"
	"sigmaos/semclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

type GroupConfig struct {
	SigmaAddrs []sp.Taddrs
	RaftAddrs  []string
}

func (cfg *GroupConfig) String() string {
	return fmt.Sprintf("&{ SigmaAddrs:%v RaftAddrs:%v }", cfg.SigmaAddrs, cfg.RaftAddrs)
}

func (cfg *GroupConfig) RaftInitialized() bool {
	for _, s := range cfg.RaftAddrs {
		if s == "" {
			return false
		}
	}
	return true
}

func newConfig(nrepl int) *GroupConfig {
	n := 1
	if nrepl > 0 {
		n = nrepl
	}
	cfg := &GroupConfig{
		SigmaAddrs: make([]sp.Taddrs, n),
		RaftAddrs:  make([]string, n),
	}
	return cfg
}

func (g *Group) readGroupConfig(path string) (*GroupConfig, error) {
	cfg := &GroupConfig{}
	err := g.GetFileJson(path, cfg)
	if err != nil {
		db.DPrintf(db.KVGRP_ERR, "Error GetFileJson: %v", err)
		return cfg, err
	}
	db.DPrintf(db.KVGRP, "readGroupConfig: %v\n", cfg)
	return cfg, nil
}

func (g *Group) writeGroupConfig(path string, cfg *GroupConfig) error {
	err := g.PutFileJsonAtomic(path, 0777, cfg)
	if err != nil {
		return err
	}
	return nil
}

func (g *Group) readCreateCfg(myid, nrepl int) *GroupConfig {
	pn := grpConfPath(g.jobdir, g.grp)
	cfg, err := g.readGroupConfig(pn)
	if err != nil { // create the initial config?
		if serr.IsErrCode(err, serr.TErrNotfound) {
			db.DPrintf(db.KVGRP, "Make initial config %v\n", pn)
			cfg = newConfig(nrepl)
			if err := g.writeGroupConfig(pn, cfg); err != nil {
				db.DFatalf("writeGroupConfig  %v err %v", pn, err)
			}
			if nrepl > 0 {
				sem := grpSemPath(g.jobdir, g.grp)
				sclnt := semclnt.NewSemClnt(g.SigmaClnt.FsLib, sem)
				sclnt.Init(0)
			}
		} else {
			db.DFatalf("readGroupConfig %v err %v", pn, err)
		}
	}
	return cfg
}

func (g *Group) AcquireReadCfg() *GroupConfig {
	g.AcquireLeadership()

	pn := grpConfPath(g.jobdir, g.grp)
	cfg, err := g.readGroupConfig(pn)
	if err != nil {
		db.DFatalf("readGroupConfig %v err %v", pn, err)
	}
	return cfg
}

func (g *Group) newRaftCfg(cfg *GroupConfig, myid, nrepl int) (*GroupConfig, *replraft.RaftConfig) {
	var raftCfg *replraft.RaftConfig

	db.DPrintf(db.KVGRP, "%v/%v newRaftConfig %v\n", g.grp, myid, cfg)

	pn := grpConfPath(g.jobdir, g.grp)

	ip := cfg.RaftAddrs[myid]
	initial := false
	if ip == "" {
		initial = true
		ip = g.ip + ":0"
	}
	raftCfg = replraft.NewRaftConfig(g.ProcEnv(), myid, ip, initial)

	if initial {
		// Get the listener address selected by raft and advertise it to group (if initial)
		cfg.RaftAddrs[myid] = raftCfg.ReplAddr()
		db.DPrintf(db.KVGRP, "%v:%v Writing cluster config: %v at %v", g.grp, myid, cfg, pn)
		if err := g.writeGroupConfig(pn, cfg); err != nil {
			db.DFatalf("registerInConfig err %v", err)
		}
		cfg = g.waitRaftConfig(cfg)
	}

	raftCfg.SetPeerAddrs(cfg.RaftAddrs)

	return cfg, raftCfg
}

// Wait until all nodes of kvgrp are up so that we the IP address to
// form an initial Raft cluster.
func (g *Group) waitRaftConfig(cfg *GroupConfig) *GroupConfig {
	db.DPrintf(db.KVGRP, "waitRaftConfig %v\n", cfg)

	sem := grpSemPath(g.jobdir, g.grp)
	sclnt := semclnt.NewSemClnt(g.SigmaClnt.FsLib, sem)

	if !cfg.RaftInitialized() {
		// Release leadership so that another member can write config
		// file with its info
		g.ReleaseLeadership()

		err := sclnt.Down()
		if err != nil {
			db.DFatalf("sem down %v err %v", sem, err)
		}
		cfg = g.AcquireReadCfg()
	} else {
		// the last one to update raft config; alert others
		sclnt.Up()
	}
	return cfg
}

func (g *Group) startServer(cfg *GroupConfig, raftCfg *replraft.RaftConfig) (*GroupConfig, error) {
	var cs any
	var err error

	cs = cachesrv.NewCacheSrv(g.ProcEnv(), "")
	if raftCfg != nil {
		cs, err = replsrv.NewReplSrv(raftCfg, cs)
		if err != nil {
			return nil, err
		}
	}

	ssrv, err := sigmasrv.NewSigmaSrvClnt("", g.SigmaClnt, cs)
	if err != nil {
		return nil, err
	}
	g.ssrv = ssrv

	cfg.SigmaAddrs[g.myid] = []*sp.Taddr{ssrv.GetEndpoint().Addrs()[0]}

	pn := grpConfPath(g.jobdir, g.grp)

	db.DPrintf(db.KVGRP, "%v/%v Writing config: %v at %v", g.grp, g.myid, cfg, pn)
	if err := g.writeGroupConfig(pn, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
