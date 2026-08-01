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
// A mapper times every step of getting a split's bytes, whatever the path does
// underneath: opening the split's reader, and then each chunk its chunk readers
// pull. So a get is
//
//   - the open, once per split: an Open RPC on the fslib path, the delegated
//     whole-window get with a cosandbox (which blocks until the prefetched
//     window materializes), and no I/O at all on the direct getput path, where
//     the reader fetches lazily;
//   - each chunk fetch: a ranged RPC to the UX/S3 proxy on the getput path
//     (plus any tail extension past the probe on the split's final chunk), a
//     streamed read on the fslib path, and a slice of the already-fetched
//     window with a cosandbox.
//
// The paths are therefore measured the same way and the totals are comparable.
// Read them with one caveat: CONCURRENCY chunk readers run in parallel per
// split, so time spent in concurrent chunk fetches is counted once per fetch and
// the sum can exceed the mapper's wall time. A cosandbox moves nearly all of a
// split's fetch into the (serial) open, which is why its total reads lower.
//
// A reducer's input is one output shard per mapper, read one at a time on every
// path, so its sums are comparable to elapsed time. Unlike the mapper's, though,
// the reducer's paths are *not* measured at the same layer, and the two are not
// directly comparable:
//
//   - getput (direct or cosandbox): the whole shard arrives at open — one
//     whole-shard RPC to the proxy on the kernel that ran the mapper, or one
//     delegated get of what the cosandbox prefetched — so the figure is the
//     fetch alone.
//   - fslib: the shard streams in as ReadKVs consumes it, through a bufio
//     reader, so there is no layer at which the fetch can be timed separately
//     without timing every buffered read. The figure therefore brackets ReadKVs
//     and includes the decode and combine interleaved with the streaming: an
//     upper bound on fetch time, not the fetch alone.
//
// So a smaller reducer figure on the getput path does not by itself mean it
// fetched faster.
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
