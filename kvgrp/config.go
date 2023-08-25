package kvgrp

import (
	"fmt"
	"time"

	"sigmaos/cachesrv"

	db "sigmaos/debug"
	"sigmaos/pathclnt"
	"sigmaos/replraft"
	"sigmaos/replsrv"
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

func (g *Group) makeRaftCfg(cfg *GroupConfig, myid, nrepl int) (*GroupConfig, *replraft.RaftConfig) {
	var raftCfg *replraft.RaftConfig

	db.DPrintf(db.KVGRP, "%v/%v makeRaftConfig %v\n", g.grp, myid, cfg)

	pn := grpConfPath(g.jobdir, g.grp)
	initial := cfg.RaftAddrs[myid] == ""
	raftCfg = replraft.MakeRaftConfig(myid, g.ip+":0", initial)

	// Get the listener address selected by raft and advertise it to group
	cfg.RaftAddrs[myid] = raftCfg.ReplAddr()
	db.DPrintf(db.KVGRP, "%v:%v Writing cluster config: %v at %v", g.grp, myid, cfg, pn)
	if err := g.writeGroupConfig(pn, cfg); err != nil {
		db.DFatalf("registerInConfig err %v", err)
	}
	return cfg, raftCfg
}

func (g *Group) startReplServer(cs any, cfg *GroupConfig, raftCfg *replraft.RaftConfig) (*replsrv.ReplSrv, *GroupConfig, error) {
	for i := 0; i < pathclnt.MAXRETRY; i++ {
		raftCfg.SetPeerAddrs(cfg.RaftAddrs)
		n := replraft.NValidAddr(cfg.RaftAddrs)
		db.DPrintf(db.KVGRP, "%v/%v peers: %d %v\n", g.grp, g.myid, n, cfg.RaftAddrs)
		if n > 1 {
			cs, err := replsrv.NewReplSrv(raftCfg, cs)
			if err == nil {
				return cs, cfg, nil
			}
			if !serr.IsErrCode(err, serr.TErrUnreachable) {
				return nil, nil, err
			}
		}
		time.Sleep(pathclnt.TIMEOUT * time.Millisecond)
		// reread config to discover hopefully more peers
		cfg = g.AcquireReadCfg()
		g.ReleaseLeadership()
	}
	return nil, nil, serr.MkErr(serr.TErrUnreachable, nil)
}

func (g *Group) startServer(cfg *GroupConfig, raftCfg *replraft.RaftConfig) (*GroupConfig, error) {
	var cs any
	var err error

	// Release leadership so that another member can start and join an
	// existing raft cfg
	g.ReleaseLeadership()

	cs = cachesrv.NewCacheSrv("")
	if raftCfg != nil {
		cs, cfg, err = g.startReplServer(cs, cfg, raftCfg)
		if err != nil {
			return nil, err
		}
	}

	ssrv, err := sigmasrv.MakeSigmaSrvClntFence("", g.SigmaClnt, cs)
	if err != nil {
		return nil, err
	}
	g.ssrv = ssrv

	cfg = g.AcquireReadCfg()

	cfg.SigmaAddrs[g.myid] = sp.MkTaddrs([]string{ssrv.MyAddr()})

	pn := grpConfPath(g.jobdir, g.grp)

	db.DPrintf(db.KVGRP, "%v/%v Writing config: %v at %v", g.grp, g.myid, cfg, pn)
	if err := g.writeGroupConfig(pn, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
