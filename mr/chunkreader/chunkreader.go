package chunkreader

import (
	"bufio"
	"bytes"
	"io"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/mr/kvmap"
	"sigmaos/mr/mr"
	"sigmaos/mr/scanner"
	"sigmaos/perf"
	sp "sigmaos/sigmap"
)

const (
	MAXCAP = 32
	MINCAP = 4
)

type ChunkReader struct {
	sbc      *scanner.ScanByteCounter
	buf      []byte
	line     []byte
	combinef mr.ReduceT
	combined *kvmap.KVMap
}

func NewChunkReader(lsz int, combinef mr.ReduceT, p *perf.Perf) *ChunkReader {
	ckr := &ChunkReader{
		sbc:      scanner.NewScanByteCounter(p),
		buf:      make([]byte, lsz),
		line:     make([]byte, lsz),
		combinef: combinef,
		combined: kvmap.NewKVMap(MINCAP, MAXCAP),
	}
	return ckr
}

func (ckr *ChunkReader) KVMap() *kvmap.KVMap {
	return ckr.combined
}

func (ckr *ChunkReader) MergeKVMap(src *ChunkReader) {
	ckr.combined.Merge(src.combined, ckr.combinef)
}

func (ckr *ChunkReader) Reset() {
	ckr.combined = kvmap.NewKVMap(MINCAP, MAXCAP)
}

func (ckr *ChunkReader) CombineEmit(emit mr.EmitT) error {
	ckr.combined.Emit(ckr.combinef, emit)
	ckr.combined = kvmap.NewKVMap(MINCAP, MAXCAP)
	return nil
}

func (ckr *ChunkReader) combine(key []byte, value string) error {
	return ckr.combined.Combine(key, value, ckr.combinef)
}

// Process a chunk from the split in parallel
func (ckr *ChunkReader) DoChunk(rdr io.Reader, o sp.Toffset, s *mr.Split, mapf mr.MapT) (sp.Tlength, error) {
	scanner := bufio.NewScanner(rdr)
	scanner.Buffer(ckr.buf, cap(ckr.buf))

	// If this is first chunk from a split with an offset, advance
	// scanner to new line after start
	n := sp.Tlength(0)
	if s.Offset != 0 && o == s.Offset {
		scanner.Scan()
		l := scanner.Bytes()
		// +1 for newline, but -1 for the extra byte we read (off-- above)
		n += sp.Tlength(len(l))
		db.DPrintf(db.MR, "%v off %v skip %d\n", s.File, s.Offset, n)
	}
	lineRdr := bytes.NewReader([]byte{})
	for scanner.Scan() {
		l := scanner.Bytes()
		n += sp.Tlength(len(l)) + 1 // 1 for newline  XXX or 2 if \r\n
		if len(l) > 0 {
			lineRdr.Reset(l)
			scan := bufio.NewScanner(lineRdr)
			scan.Buffer(ckr.line, cap(ckr.line))
			scan.Split(ckr.sbc.ScanWords)
			if err := mapf(s.File, scan, ckr.combine); err != nil {
				return 0, err
			}
		}
		if n >= s.Length {
			db.DPrintf(db.MR, "%v read %v bytes %d extra %d", s.File, n, s.Length, n-s.Length)
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return sp.Tlength(n), err
	}
	return n, nil
}

func (ckr *ChunkReader) ChunkReader(pfr *fslib.ParallelFileReader, s *mr.Split, mapf mr.MapT) (sp.Tlength, error) {
	t := sp.Tlength(0)
	for {
		rdr, o, err := pfr.GetChunkReader(cap(ckr.buf))
		if err != nil && err == io.EOF {
			break
		}
		n, err := ckr.DoChunk(rdr, o, s, mapf)
		t += n
		if err != nil {
			return t, err
		}
	}
	return t, nil
}
