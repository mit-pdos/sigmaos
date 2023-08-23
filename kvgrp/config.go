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

func (g *Group) registerInConfig(myid, nrepl int) (*GroupConfig, *replraft.RaftConfig) {
	pn := grpConfPath(g.jobdir, g.grp)
	cfg, err := g.readGroupConfig(pn)
	if err != nil {
		if serr.IsErrCode(err, serr.TErrNotfound) {
			db.DPrintf(db.KVGRP, "Make initial config %v\n", pn)
			cfg = newConfig(nrepl)
			if nrepl > 0 {
				sem := grpSemPath(g.jobdir, g.grp)
				sclnt := semclnt.MakeSemClnt(g.SigmaClnt.FsLib, sem)
				sclnt.Init(0)
			}
		} else {
			db.DFatalf("Unexpected config %v error %v", pn, err)
		}
	}

	var raftCfg *replraft.RaftConfig
	initial := false
	if nrepl > 0 && cfg.RaftAddrs[myid] == "" {
		raftCfg = replraft.MakeRaftConfig(myid, g.ip+":0", true)
		// Get the listener address selected by the raft library.
		cfg.RaftAddrs[myid] = raftCfg.ReplAddr()
		initial = true
	}

	db.DPrintf(db.KVGRP, "%v:%v Writing cluster config: %v at %v", g.grp, myid, cfg, pn)

	if err := g.writeGroupConfig(pn, cfg); err != nil {
		db.DFatalf("registerInConfig err %v", err)
	}

	if initial {
		cfg = g.waitRaftConfig(cfg)
		raftCfg.SetPeerAddrs(cfg.RaftAddrs)
	}

	return cfg, raftCfg
}

// Wait until initil raft cluster has configured itself
func (g *Group) waitRaftConfig(cfg *GroupConfig) *GroupConfig {

	db.DPrintf(db.KVGRP, "waitRaftConfig %v\n", cfg)

	sem := grpSemPath(g.jobdir, g.grp)
	sclnt := semclnt.MakeSemClnt(g.SigmaClnt.FsLib, sem)

	if !cfg.RaftInitialized() {

		// Release leadership so that another member can write config
		// file with its info
		g.ReleaseLeadership()

		err := sclnt.Down()
		if err != nil {
			db.DFatalf("sem down %v err %v", sem, err)
		}

		g.AcquireLeadership()

		pn := grpConfPath(g.jobdir, g.grp)
		cfg, err = g.readGroupConfig(pn)
		if err != nil {
			db.DFatalf("readGroupConfig %v err %v", pn, err)
		}

	} else {
		// the last one to update raft config; alert others
		sclnt.Up()
	}
	return cfg
}

// Must run after SetPeerAddrs()
func (g *Group) startServer(cfg *GroupConfig, raftCfg *replraft.RaftConfig) {
	var cs any
	cs = cachesrv.NewCacheSrv("")
	if raftCfg != nil {
		cs = replsrv.NewReplSrv(raftCfg, cs)
	}

	ssrv, err := sigmasrv.MakeSigmaSrvClntFence("", g.SigmaClnt, cs)
	if err != nil {
		db.DFatalf("MakeSigmaSrvClnt %v\n", err)
	}
	g.ssrv = ssrv

	cfg.SigmaAddrs[g.myid] = sp.MkTaddrs([]string{ssrv.MyAddr()})

	pn := grpConfPath(g.jobdir, g.grp)

	db.DPrintf(db.KVGRP, "%v:%v Writing cluster config: %v at %v", g.grp, g.myid, cfg, pn)
	if err := g.writeGroupConfig(pn, cfg); err != nil {
		db.DFatalf("registerInConfig err %v", err)
	}
}
