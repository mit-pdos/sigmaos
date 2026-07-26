package coord

import (
	"encoding/binary"
	"strconv"

	"sigmaos/apps/mr"
	mrapi "sigmaos/apps/mr/mr"
	"sigmaos/proxy/getput"
	wasmer "sigmaos/proxy/wasm/rpc/wasmer"
	sp "sigmaos/sigmap"
)

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
