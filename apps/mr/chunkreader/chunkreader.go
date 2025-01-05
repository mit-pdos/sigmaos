package chunkreader

import (
	"bufio"
	"bytes"
	"io"

	"sigmaos/apps/mr/kvmap"
	"sigmaos/apps/mr/mr"
	mrscanner "sigmaos/apps/mr/scanner"
	db "sigmaos/debug"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

const (
	MAXCAP = 32
	MINCAP = 4
)

type ChunkReader struct {
	sbc      *mrscanner.ScanByteCounter
	buf      []byte
	line     []byte
	wsz      int
	combinef mr.ReduceT
	combined *kvmap.KVMap
}

func NewChunkReader(lsz, wsz int, combinef mr.ReduceT, p *perf.Perf) *ChunkReader {
	sz := lsz + wsz
	ckr := &ChunkReader{
		sbc:      mrscanner.NewScanByteCounter(p),
		buf:      make([]byte, sz),
		line:     make([]byte, sz),
		wsz:      wsz,
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
	err := ckr.combined.Emit(ckr.combinef, emit)
	ckr.combined = kvmap.NewKVMap(MINCAP, MAXCAP)
	return err
}

func (ckr *ChunkReader) combine(key []byte, value string) error {
	return ckr.combined.Combine(key, value, ckr.combinef)
}

// Process a chunk from the split in parallel
func (ckr *ChunkReader) DoChunk(rdr io.Reader, o sp.Toffset, s *mr.Split, mapf mr.MapT) (sp.Tlength, error) {
	scanner := bufio.NewScanner(rdr)
	scanner.Buffer(ckr.buf, cap(ckr.buf))

	db.DPrintf(db.MR, "DoChunk off %d %v", o, s)
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
	skip := o > s.Offset
	e := sp.Tlength(cap(ckr.line) - ckr.wsz)
	for scanner.Scan() {
		l := scanner.Bytes()
		l0 := sp.Tlength(len(l) + 1) // 1 for newline  XXX or 2 if \r\n

		// if this isn't the first chunk and this line is the first
		// one of chunk, skip to separator
		if skip {
			skip = false
			start, _ := mrscanner.ScanSeperator(l)
			l = l[start:]
		}
		if l0 > 1 {
			if n+l0 > e {
				// scan to first separator beyond linesz
				start := int(e - n)
				end, _ := mrscanner.ScanSeperator(l[start:])
				l = l[0 : start+end]

			}
			lineRdr.Reset(l)
			scan := bufio.NewScanner(lineRdr)
			scan.Buffer(ckr.line, cap(ckr.line))
			scan.Split(ckr.sbc.ScanWords)
			if err := mapf(s.File, scan, ckr.combine); err != nil {
				return 0, err
			}
		}
		n += l0
		if n-1 >= sp.Tlength(e) {
			return n, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return sp.Tlength(n), err
	}
	return n, nil
}

func (ckr *ChunkReader) ReadChunks(pfr *fslib.ParallelFileReader, s *mr.Split, mapf mr.MapT) (sp.Tlength, error) {
	t := sp.Tlength(0)
	for {
		rdr, o, err := pfr.GetChunkReader(cap(ckr.buf), cap(ckr.buf)-ckr.wsz)
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
