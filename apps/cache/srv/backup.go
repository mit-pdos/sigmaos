package srv

import (
	"path/filepath"

	"sigmaos/apps/cache"
	"sigmaos/apps/cache/cachegrp"
	cacheclnt "sigmaos/apps/cache/clnt"
	db "sigmaos/debug"
	"sigmaos/proc"
)

func RunCacheSrvBackup(cachedir, jobname, shardpn string, nshard int) error {
	pe := proc.GetProcEnv()
	s, err := NewCacheSrv(pe, cachedir, shardpn, nshard)
	if err != nil {
		return err
	}
	// Get peer name
	peer := filepath.Base(shardpn)
	peerpn := cachedir + cachegrp.Server(peer)
	db.DPrintf(db.CACHESRV, "Peer name: %v", peer)
	cc := cacheclnt.NewCacheClnt(s.ssrv.SigmaClnt().FsLib, jobname, nshard)
	// Dump peer shards
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
	db.DPrintf(db.CACHESRV, "Loaded cache state from peer %v", peerpn)
	// Run server
	s.ssrv.RunServer()
	s.exitCacheSrv()
	return nil
}
