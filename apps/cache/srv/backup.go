package srv

import (
	"path/filepath"

	"sigmaos/apps/cache"
	"sigmaos/apps/cache/cachegrp"
	cacheclnt "sigmaos/apps/cache/clnt"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/rpc"
	sp "sigmaos/sigmap"
)

const (
	GET_ALL_SHARDS = 0
)

func RunCacheSrvBackup(cachedir, jobname, shardpn string, nshard int, useEPCache bool, topN int) error {
	pe := proc.GetProcEnv()
	s, err := NewCacheSrv(pe, cachedir, shardpn, nshard, useEPCache)
	if err != nil {
		return err
	}
	// Get peer name
	peer := filepath.Base(shardpn)
	peerpn := cachedir + cachegrp.Server(peer)
	db.DPrintf(db.CACHESRV, "Peer name: %v", peer)
	cc := cacheclnt.NewCacheClnt(s.ssrv.SigmaClnt().FsLib, jobname, nshard)
	if topN != GET_ALL_SHARDS {
		db.DFatalf("unimplemented")
		// TODO:
		// 1. get hot shard list
		// 2. get hot shards
	}
	// If not doing delegated initialization, fetch directly from peer
	if !pe.GetRunBootScript() {
		ep, _ := pe.GetCachedEndpoint(peerpn)
		// First, mount the peer
		if err := s.ssrv.SigmaClnt().MountTree(ep, rpc.RPC, filepath.Join(peerpn, rpc.RPC)); err != nil {
			db.DFatalf("Err mount peer: %v", err)
			return err
		}
		// Dump peer shards via direct RPC
		for i := 0; i < nshard; i++ {
			shard := cache.Tshard(i)
			vals, err := cc.DumpShard(peerpn, shard, sp.NullFence())
			if err != nil {
				db.DFatalf("Err DumpShard(%v) from server %v: %v", i, peer, err)
			}
			if err := s.loadShard(shard, vals); err != nil {
				db.DFatalf("Err LoadShard(%v) from server %v: %v", i, peer, err)
			}
		}
	} else {
		// Dump peer shards via delegated RPC
		for i := 0; i < nshard; i++ {
			shard := cache.Tshard(i)
			vals, err := cc.DelegatedDumpShard(peerpn, i)
			if err != nil {
				db.DFatalf("Err DumpShard(%v) from server %v: %v", i, peer, err)
			}
			if err := s.loadShard(shard, vals); err != nil {
				db.DFatalf("Err LoadShard(%v) from server %v: %v", i, peer, err)
			}
		}
	}
	db.DPrintf(db.CACHESRV, "Loaded cache state from peer %v", peerpn)
	// Run server
	s.ssrv.RunServer()
	s.exitCacheSrv()
	return nil
}
