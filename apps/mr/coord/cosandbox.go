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

// mapperShmemMB sizes the shared-memory segment spproxy sets up for a
// cosandbox mapper at twice the job's bin size, rounded up to whole MB.
// spproxy reads each prefetched read window into the segment and the mapper's
// delegated get maps it there instead of copying it back over the spproxy
// socket, so the segment has to hold all of a mapper's windows at once (the
// boot script issues one get per split). mr.NewBins closes a bin once adding
// another split would reach binsz, so binsz bounds a bin's data; the factor of
// two covers the per-split tail probes, the marshaled reply framing, and the
// allocator's slack.
func mapperShmemMB(binsz int) proc.Tmem {
	mb := proc.Tmem((2*uint64(binsz) + uint64(sp.MBYTE) - 1) / uint64(sp.MBYTE))
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
func mapperBootInput(bin mr.Bin, linesz, probesz int) ([]byte, error) {
	strs := make([]string, 0, 1+5*len(bin))
	strs = append(strs, sp.LOCAL)
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
func reducerShmemMB(cfgMB int, mapOutBytes int64, nreduce int) proc.Tmem {
	if cfgMB > 0 {
		return proc.Tmem(cfgMB)
	}
	if nreduce < 1 {
		nreduce = 1
	}
	perReducer := uint64(mapOutBytes) / uint64(nreduce)
	mb := proc.Tmem((2*perReducer + uint64(sp.MBYTE) - 1) / uint64(sp.MBYTE))
	if mb < MIN_SHMEM_MB {
		return MIN_SHMEM_MB
	}
	return mb
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
