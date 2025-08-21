package srv

import (
	"path/filepath"
	"time"

	"sigmaos/apps/cache/cachegrp"
	cacheclnt "sigmaos/apps/cache/clnt"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/rpc"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

const (
	GET_ALL_SHARDS = 0
)

func RunCacheSrvBackup(cachedir, jobname, shardpn string, nshard int, useEPCache bool, topN int) error {
	pe := proc.GetProcEnv()
	start := time.Now()
	s, err := NewCacheSrv(pe, cachedir, shardpn, nshard, useEPCache)
	if err != nil {
		return err
	}
	perf.LogSpawnLatency("Backup.NewCacheSrv", pe.GetPID(), pe.GetSpawnTime(), start)
	// Get peer name
	peer := filepath.Base(shardpn)
	peerpn := cachedir + cachegrp.Server(peer)
	db.DPrintf(db.CACHESRV, "Peer name: %v", peer)
	start = time.Now()
	cc := cacheclnt.NewCacheClnt(s.ssrv.SigmaClnt().FsLib, jobname, nshard, true)
	perf.LogSpawnLatency("Backup.NewCacheClnt", pe.GetPID(), pe.GetSpawnTime(), start)
	// We only need to mount the primary if we are not running the boot script
	// (otherwise the RPC client can be lazily initialized)
	if !pe.GetRunBootScript() {
		start = time.Now()
		ep, _ := pe.GetCachedEndpoint(peerpn)
		// First, mount the peer
		if err := s.ssrv.SigmaClnt().MountTree(ep, rpc.RPC, filepath.Join(peerpn, rpc.RPC)); err != nil {
			db.DFatalf("Err mount peer: %v", err)
			return err
		}
		perf.LogSpawnLatency("Backup.MountPrimary", pe.GetPID(), pe.GetSpawnTime(), start)
	}
	start = time.Now()
	// If not doing delegated initialization, fetch directly from peer
	if !pe.GetRunBootScript() {
		hotShards, _, err := cc.GetHotShards(peerpn, uint32(topN))
		if err != nil {
			db.DFatalf("Err GetHotShards: %v", err)
		}
		db.DPrintf(db.CACHESRV, "top %v hot shards: %v", topN, hotShards)
		// Dump peer shards via direct RPC
		for _, shard := range hotShards {
			vals, err := cc.DumpShard(peerpn, shard, sp.NullFence())
			if err != nil {
				db.DFatalf("Err DumpShard(%v) from server %v: %v", shard, peer, err)
			}
			if err := s.loadShard(shard, vals); err != nil {
				db.DFatalf("Err LoadShard(%v) from server %v: %v", shard, peer, err)
			}
		}
	} else {
		hotShards, _, err := cc.DelegatedGetHotShards(peerpn, 0)
		if err != nil {
			db.DFatalf("Err DelegatedGetHotShards: %v", err)
		}
		db.DPrintf(db.CACHESRV, "top %v delegated hot shards(%v): %v", topN, len(hotShards), hotShards)
		// Dump peer shards via delegated RPC
		for i, shard := range hotShards {
			// First RPC is the GetHotShards RPC
			rpcIdx := i + 1
			vals, err := cc.DelegatedDumpShard(peerpn, rpcIdx)
			if err != nil {
				db.DFatalf("Err DumpShard(%v) from server %v: %v", rpcIdx, peer, err)
			}
			if err := s.loadShard(shard, vals); err != nil {
				db.DFatalf("Err LoadShard(%v) from server %v: %v", rpcIdx, peer, err)
			}
		}
	}
	perf.LogSpawnLatency("Backup.LoadCacheState", pe.GetPID(), pe.GetSpawnTime(), start)
	if !db.WillBePrinted(db.SPAWN_LAT) {
		db.DPrintf(db.ALWAYS, "LoadCacheState & ready to run e2e %v op %v", time.Since(pe.GetSpawnTime()), start)
	}
	db.DPrintf(db.CACHESRV, "Loaded cache state from peer %v", peerpn)
	// Run server
	s.ssrv.RunServer()
	s.exitCacheSrv()
	return nil
}
