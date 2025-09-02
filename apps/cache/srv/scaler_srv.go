package srv

import (
	"path/filepath"
	"strconv"
	"time"

	"sigmaos/apps/cache"
	cacheclnt "sigmaos/apps/cache/clnt"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/rpc"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

func RunCacheSrvScaler(cachedir, jobname, srvpn string, nshard int, oldNSrv int, newNSrv int, useEPCache bool) error {
	pe := proc.GetProcEnv()
	start := time.Now()
	s, err := NewCacheSrv(pe, cachedir, srvpn, nshard, useEPCache)
	if err != nil {
		return err
	}
	perf.LogSpawnLatency("Scaler.NewCacheSrv", pe.GetPID(), pe.GetSpawnTime(), start)
	// Get peer name
	srvIDStr := srvpn
	srvID, err := strconv.Atoi(srvIDStr)
	if err != nil {
		db.DFatalf("Err convert srv ID to int: %v", err)
		return err
	}
	// Map of servers to steal shards from, and the list of shards to steal from
	// each server
	shardsToSteal := make(map[int][]int)
	srcSrvs := make([]int, 0, oldNSrv)
	rpcIdxs := []uint64{}
	for i := 0; i < oldNSrv; i++ {
		srcSrvs = append(srcSrvs, i)
		shardsToSteal[i] = []int{}
	}
	nrpc := uint64(0)
	for i := 0; i < nshard; i++ {
		if i%newNSrv == srvID {
			// If this server should host the shard in the new configuration, try to
			// steal it
			srcSrv := i % oldNSrv
			// Add this shard to the list of shards to steal from the source server
			shardsToSteal[srcSrv] = append(shardsToSteal[srcSrv], i)
			rpcIdxs = append(rpcIdxs, nrpc)
			nrpc++
		}
	}
	start = time.Now()
	cc := cacheclnt.NewCacheClnt(s.ssrv.SigmaClnt().FsLib, jobname, nshard, true)
	perf.LogSpawnLatency("Scaler.NewCacheClnt", pe.GetPID(), pe.GetSpawnTime(), start)
	// We only need to mount the primary if we are not running the boot script
	// (otherwise the RPC client can be lazily initialized)
	if !pe.GetRunBootScript() {
		start = time.Now()
		for _, srcSrv := range srcSrvs {
			peerpn := filepath.Join(cachedir, strconv.Itoa(srcSrv))
			ep, ok := pe.GetCachedEndpoint(peerpn)
			if !ok {
				db.DFatalf("Missing cached EP for peer %v:\n%v", peerpn, pe)
			}
			// First, mount the peers
			if err := s.ssrv.SigmaClnt().MountTree(ep, rpc.RPC, filepath.Join(peerpn, rpc.RPC)); err != nil {
				db.DFatalf("Err mount peer: %v", err)
				return err
			}
		}
		perf.LogSpawnLatency("Scaler.MountSrcSrvs", pe.GetPID(), pe.GetSpawnTime(), start)
	}
	start = time.Now()
	// If not doing delegated initialization, fetch directly from peer
	if !pe.GetRunBootScript() {
		// For each source server, dump shards to be stolen
		for _, srcSrv := range srcSrvs {
			peerpn := filepath.Join(cachedir, strconv.Itoa(srcSrv))
			// Dump shards from source server via direct RPC
			for _, shard := range shardsToSteal[srcSrv] {
				vals, err := cc.DumpShard(peerpn, cache.Tshard(shard), sp.NullFence(), false)
				if err != nil {
					db.DFatalf("Err DumpShard(%v) from server %v: %v", shard, peerpn, err)
				}
				if err := s.loadShard(cache.Tshard(shard), vals); err != nil {
					db.DFatalf("Err LoadShard(%v) from server %v: %v", shard, srcSrv, err)
				}
			}
		}
	} else {
		if err := cc.BatchFetchDelegatedRPCs(filepath.Join(cachedir, strconv.Itoa(0)), rpcIdxs, 2*len(rpcIdxs)); err != nil {
			db.DFatalf("Err batch-fetching delegated RPCs: %v", err)
		}
		rpcIdx := 0
		for _, srcSrv := range srcSrvs {
			peerpn := filepath.Join(cachedir, strconv.Itoa(srcSrv))
			// Dump shards from source server via delegated RPC
			for _, shard := range shardsToSteal[srcSrv] {
				vals, err := cc.DelegatedDumpShard(peerpn, rpcIdx)
				if err != nil {
					db.DFatalf("Err DumpShard(%v) from server %v: %v", rpcIdx, peerpn, err)
				}
				if err := s.loadShard(cache.Tshard(shard), vals); err != nil {
					db.DFatalf("Err LoadShard(%v) from server %v: %v", rpcIdx, srcSrv, err)
				}
				rpcIdx++
			}
		}
	}
	perf.LogSpawnLatency("Scaler.LoadCacheState", pe.GetPID(), pe.GetSpawnTime(), start)
	if !db.WillBePrinted(db.SPAWN_LAT) {
		db.DPrintf(db.ALWAYS, "LoadCacheState & ready to run e2e %v op %v", time.Since(pe.GetSpawnTime()), start)
	}
	db.DPrintf(db.CACHESRV, "Loaded cache state shards %v", shardsToSteal)
	// Run server
	s.ssrv.RunServer()
	s.exitCacheSrv()
	return nil
}
