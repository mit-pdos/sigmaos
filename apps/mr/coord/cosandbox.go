package coord

import (
	"encoding/binary"
	"strconv"

	"sigmaos/apps/mr"
	mrapi "sigmaos/apps/mr/mr"
	"sigmaos/proc"
	"sigmaos/proxy/getput"
	wasmer "sigmaos/proxy/wasm/rpc/wasmer"
	sp "sigmaos/sigmap"
)

// Bytes of reply framing spproxy allocates in the segment alongside each
// prefetched window: the marshaled rpcproto.Rep and the reply message, which
// measure 2 and 4 bytes for an S3/UX get. Rounded well up, since it is per
// split and the cost of being wrong is a job that dies mid-map.
const shmemReplyFramingBytes = 1024

// Headroom over the computed requirement. A segment is a per-proc reservation
// against the node's /dev/shm, so this is deliberately small: oversizing it
// costs every concurrently running mapper on the node.
const SHMEM_MARGIN_MB proc.Tmem = 8

// mapperShmemMB sizes the shared-memory segment spproxy sets up for a cosandbox
// mapper. spproxy reads each prefetched read window into the segment and the
// mapper's delegated get maps it there instead of copying it back over the
// spproxy socket, so the segment has to hold all of a mapper's windows at once
// (the boot script issues one get per split).
//
// mr.NewBins closes a bin once adding another split would reach binsz, so binsz
// bounds a bin's data; on top of that each split carries a tail probe and its
// reply framing. Sized to that requirement plus a small margin, and no more: the
// segment is memory reserved from the node's /dev/shm (400MB, see
// dcontainer.go) and every mapper running concurrently on the node holds one.
// A previous 2x-binsz fudge asked for 260MB per mapper where 120MB was used,
// which with 2 mappers per node oversubscribed /dev/shm.
func mapperShmemMB(binsz, nsplit, linesz, probesz int) proc.Tmem {
	// The probe is the same for every split of a job: SplitReadWindow defaults
	// and caps it without reference to the split.
	_, _, probe := mrapi.SplitReadWindow(&mrapi.Split{}, linesz, probesz)
	need := uint64(binsz) + uint64(nsplit)*(uint64(probe)+shmemReplyFramingBytes)
	return shmemMB(need)
}

// shmemMB rounds a byte requirement up to whole MB, adds the margin, and
// enforces the floor.
func shmemMB(need uint64) proc.Tmem {
	mb := proc.Tmem((need+uint64(sp.MBYTE)-1)/uint64(sp.MBYTE)) + SHMEM_MARGIN_MB
	if mb < MIN_SHMEM_MB {
		return MIN_SHMEM_MB
	}
	return mb
}

// mapperBootInput builds the boot input for the mr_mapper_boot cosandbox
// (see rs/wasm/mr_mapper_boot): a u32-LE split count followed by
// wasmer.EncodeArgs([kid, typ_i, a_i, b_i, off_i, cnt_i, ...]). The boot
// script issues one ranged get per split, in bin order, at rpcIdx = split
// index — the same order (and, via mr.SplitReadWindow, the same window)
// the mapper's GetPutReader uses to retrieve them. If the two ever
// diverge, the mapper hangs (missing rpcIdx) or maps corrupted boundaries.
// uxKid is the UX server the gets go to: a dedicated machine's kernel ID when the
// coordinator has pointed this mapper at one, or "" for the cosandbox's own node.
// It has to match what the splits name, or the boot script fetches from a server
// that hasn't got the file — the input is staged only on the dedicated machines,
// so "fetch this path from your local server" finds nothing there.
func mapperBootInput(bin mr.Bin, linesz, probesz int, uxKid string) ([]byte, error) {
	kid := uxKid
	if kid == "" {
		kid = sp.LOCAL
	}
	strs := make([]string, 0, 1+5*len(bin))
	strs = append(strs, kid)
	for i := range bin {
		s := &bin[i]
		tgt, err := getput.ClassifyPath(s.File)
		if err != nil {
			return nil, err
		}
		off, body, probe := mrapi.SplitReadWindow(s, linesz, probesz)
		cnt := uint64(body) + uint64(probe)
		offStr := strconv.FormatUint(uint64(off), 10)
		cntStr := strconv.FormatUint(cnt, 10)
		if tgt.Kind == getput.TS3 {
			strs = append(strs, "s3", tgt.Bucket, tgt.Key, offStr, cntStr)
		} else {
			strs = append(strs, "ux", tgt.Path, "", offStr, cntStr)
		}
	}
	input := make([]byte, 4, 4+16*len(strs))
	binary.LittleEndian.PutUint32(input, uint32(len(bin)))
	return append(input, wasmer.EncodeArgs(strs)...), nil
}

// reducerShmemMB sizes the shared-memory segment spproxy sets up for a
// cosandbox reducer. Every shard the boot script prefetches is resident in the
// segment at once, so unlike a mapper — whose prefetch is bounded by one bin —
// a reducer's segment scales with the whole job's intermediate size, which is
// why the job description carries it (mr.Job.ReduceShmemMB).
//
// cfgMB is that setting. When it is 0, fall back to twice the intermediate
// output one reducer reads — what the mappers actually wrote, divided by the
// number of reducers, which the coordinator knows by the time it spawns any
// reducer. That keeps a job description that doesn't set it working, but the
// configured value is what to reach for when a job's per-reducer input is known
// ahead of time: it is requested memory, so both under- and oversizing it cost.
func reducerShmemMB(cfgMB, nshard int, mapOutBytes int64, nreduce int) proc.Tmem {
	if cfgMB > 0 {
		return proc.Tmem(cfgMB)
	}
	if nreduce < 1 {
		nreduce = 1
	}
	// What one reducer reads, plus the framing of one reply per shard. Sized to
	// the requirement rather than a multiple of it, for the same reason as
	// mapperShmemMB: it is a reservation against the node's /dev/shm.
	need := uint64(mapOutBytes)/uint64(nreduce) + uint64(nshard)*shmemReplyFramingBytes
	return shmemMB(need)
}

// reducerBootInput builds the boot input for the mr_reducer_boot cosandbox
// (see rs/wasm/mr_reducer_boot): a u32-LE shard count followed by
// wasmer.EncodeArgs([typ_i, kid_i, a_i, b_i, ...]). The boot script issues one
// whole-file get per shard, in bin order, at rpcIdx = shard index — the same
// index Reducer.readFile uses to retrieve it. Unlike the mapper's manifest each
// entry carries its own kernel ID, since a reducer's shards are spread across
// whichever kernels ran the mappers.
func reducerBootInput(bin mr.Bin) ([]byte, error) {
	strs := make([]string, 0, 4*len(bin))
	for i := range bin {
		tgt, err := getput.ClassifyPath(bin[i].File)
		if err != nil {
			return nil, err
		}
		// A shard path that doesn't name a kernel (an S3 path still holding
		// ~local, which Mapper.outputBin leaves alone) is served by the proxy
		// on the reducer's own kernel, which is what sp.LOCAL resolves to in
		// the cosandbox.
		kid := tgt.Kid
		if kid == "" {
			kid = sp.LOCAL
		}
		if tgt.Kind == getput.TS3 {
			strs = append(strs, "s3", kid, tgt.Bucket, tgt.Key)
		} else {
			strs = append(strs, "ux", kid, tgt.Path, "")
		}
	}
	input := make([]byte, 4, 4+16*len(strs))
	binary.LittleEndian.PutUint32(input, uint32(len(bin)))
	return append(input, wasmer.EncodeArgs(strs)...), nil
}
