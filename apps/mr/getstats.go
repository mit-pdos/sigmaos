package mr

import (
	"sync/atomic"
	"time"
)

// getStats accumulates the wall time a task spends fetching its input, so that
// it can be read against the task's total runtime (Result.MsInner) independently
// of how the input is fetched. What counts as one get differs by path, and so
// does how the sum relates to elapsed time.
//
// A mapper's input is its bin of splits:
//
//   - getput, cosandbox: one delegated get per split, which retrieves the
//     window the cosandbox prefetched (blocking until it materializes).
//   - getput, direct: one ranged get RPC per split to the local UX/S3 proxy,
//     plus any lazy tail extension past the probe.
//   - fslib: one chunk read. CONCURRENCY chunk readers run in parallel per
//     split, so the sum counts concurrent reads separately and can exceed the
//     mapper's wall time.
//
// A reducer's input is one output shard per mapper:
//
//   - getput, cosandbox: one delegated whole-shard get per mapper.
//   - getput, direct: one whole-shard get RPC per mapper, to the proxy on the
//     kernel that ran it.
//   - fslib: one streamed read per shard, timed around the ReadKVs that
//     consumes it.
//
// Every path but the mapper's fslib one issues its gets serially, so their sums
// are comparable to elapsed time. Compare across paths with that in mind.
type getStats struct {
	ns atomic.Int64
	n  atomic.Int64
}

func (gs *getStats) record(start time.Time) {
	gs.ns.Add(int64(time.Since(start)))
	gs.n.Add(1)
}

func (gs *getStats) dur() time.Duration {
	return time.Duration(gs.ns.Load())
}

func (gs *getStats) count() int64 {
	return gs.n.Load()
}
