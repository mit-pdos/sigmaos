package remote

// Commands used to start benchmarks

const (
	InitFSCmd string = "go clean -testcache; " +
		"go test -v sigmaos/fslib -timeout 0 InitFs " +
		"> /tmp/bench.out 2>&1"
)
