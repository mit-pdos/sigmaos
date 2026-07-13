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

// Process a chunk from the split in parallel. o is the chunk's absolute
// offset in the file. final indicates this chunk contains the split's last
// byte: it is processed line-granularly through the end of the line that
// straddles the split end (its reader supplies those bytes), instead of
// stopping word-granularly at the chunk quota.
func (ckr *ChunkReader) DoChunk(rdr io.Reader, o sp.Toffset, final bool, s *mr.Split, mapf mr.MapT) (sp.Tlength, error) {
	scanner := bufio.NewScanner(rdr)
	scanner.Buffer(ckr.buf, cap(ckr.buf))

	db.DPrintf(db.MR, "DoChunk off %d final %t %v", o, final, s)
	// If this is the first chunk from a split with an offset, advance the
	// scanner to the new line after start. The chunk starts at s.Offset-1
	// (doSplit reads one byte early): if that byte is a newline the first
	// token is empty and nothing is skipped; otherwise the first token is
	// the tail of a line the previous split's mapper processed.
	n := sp.Tlength(0)
	pos := o // absolute offset of the next unconsumed byte
	if s.Offset != 0 && o == s.Offset-1 {
		scanner.Scan()
		l := scanner.Bytes()
		// +1 for the newline. n must count raw bytes consumed from the
		// chunk's start o: the chunk quota boundary o+e is where the next
		// chunk starts, so an off-by-one here shifts the quota trim
		// relative to the next chunk's leading-word skip and words at the
		// boundary get double-counted or lost.
		n += sp.Tlength(len(l) + 1)
		pos += sp.Toffset(len(l)) + 1
		db.DPrintf(db.MR, "%v off %v skip %d\n", s.File, s.Offset, n)
	}
	lineRdr := bytes.NewReader([]byte{})
	skip := o > s.Offset
	e := sp.Tlength(cap(ckr.line) - ckr.wsz)
	splitEnd := s.Offset + sp.Toffset(s.Length)
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
		if l0 > 1 && len(l) > 0 {
			if !final && n+l0 > e {
				// scan to first separator beyond the chunk quota
				start := 0
				if e > n {
					start = int(e - n)
					if start > len(l) {
						start = len(l)
					}
				}
				end, _ := mrscanner.ScanSeperator(l[start:])
				l = l[0 : start+end]
			}
			if len(l) > 0 {
				lineRdr.Reset(l)
				scan := bufio.NewScanner(lineRdr)
				scan.Buffer(ckr.line, cap(ckr.line))
				scan.Split(ckr.sbc.ScanWords)
				if err := mapf(s.File, scan, ckr.combine); err != nil {
					return 0, err
				}
			}
		}
		n += l0
		pos += sp.Toffset(l0)
		if final {
			// Stop after the line containing the split's last byte
			if pos >= splitEnd {
				return n, nil
			}
		} else if n-1 >= sp.Tlength(e) {
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
		rdr, o, final, err := pfr.GetChunkReader(cap(ckr.buf), cap(ckr.buf)-ckr.wsz)
		if err == io.EOF {
			break
		}
		if err != nil {
			return t, err
		}
		n, err := ckr.DoChunk(rdr, o, final, s, mapf)
		t += n
		if err != nil {
			return t, err
		}
	}
	return t, nil
}
