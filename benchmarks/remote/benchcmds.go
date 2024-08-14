package remote

import (
	"fmt"
)

// Constructors for commands used to start benchmarks

func GetInitFSCmd(bcfg *BenchConfig, ccfg *ClusterConfig) string {
	const (
		debugSelectors string = "\"BENCH;TEST;\""
	)
	return fmt.Sprintf("export SIGMADEBUG=%s; go clean -testcache; "+
		"go test -v sigmaos/fslib -timeout 0 --no-shutdown --etcdIP %s --tag %s "+
		"--run InitFs "+
		"> /tmp/bench.out 2>&1",
		debugSelectors,
		ccfg.LeaderNodeIP,
		bcfg.Tag,
	)
}

func GetColdStartCmd(bcfg *BenchConfig, ccfg *ClusterConfig) string {
	const (
		debugSelectors string = "\"TEST;BENCH;LOADGEN;SPAWN_LAT;NET_LAT;REALM_GROW_LAT;CACHE_LAT;WALK_LAT;FSETCD_LAT;ATTACH_LAT;CHUNKSRV;CHUNKCLNT;\""
	)
	return fmt.Sprintf("export SIGMADEBUG=%s; go clean -testcache; "+
		"go test -v sigmaos/benchmarks -timeout 0 --no-shutdown --etcdIP %s --tag %s "+
		"--run TestMicroScheddSpawn "+
		"--use_rust_proc "+
		"--schedd_dur 5s "+
		"--schedd_max_rps 8 "+
		"> /tmp/bench.out 2>&1",
		debugSelectors,
		ccfg.LeaderNodeIP,
		bcfg.Tag,
	)
}
