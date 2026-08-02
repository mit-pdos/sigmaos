package getput

import (
	"bytes"
	"io"
	"sync"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

// Fallback tail-probe size, for a caller that passes none. A zero probe would
// otherwise reach getChunk as a zero count, which means "read to EOF" at both
// proxies — a whole-file fetch instead of a probe.
const minTailProbe = 4096

// GetPutReader is a SplitReader over a split's read window, [off, off+body),
// which the mapper's chunk readers tile in sz/offinc-sized chunks.
//
// Direct: each GetChunkReader call fetches its own chunk with a ranged RPC to
// the proxy, exactly as fslib's ParallelFileReader does with PreadRdr. That is
// what lets the mapper's CONCURRENCY chunk readers overlap fetching with
// mapping, and what puts several ranged gets in flight per split rather than
// one. Fetching the whole window up front instead cost ~210ms of dead time per
// split and left four of the five chunk readers idle (see
// claude-slop/GET_PUT_SLOW.md).
//
// Delegated: a cosandbox prefetched the whole window under this split's rpcIdx
// (the store holds exactly one reply per idx, so the fetch cannot be
// subdivided — the coordinator's manifest already subdivides by issuing one get
// per split). The window is tiled all the same, out of the buffer already in
// memory: tiling is what spreads a split across the mapper's CONCURRENCY chunk
// readers, and serving it as one chunk left the other readers with nothing to
// do, mapping each split on a single core. Same chunk arithmetic as the direct
// path, minus the fetch.
//
// Either way the final chunk extends past the split end through the first
// newline, so the line straddling the boundary can be finished; DoChunk's
// final-chunk path processes line-granularly to the split end, so surplus probe
// bytes need no trimming. Tail extensions are always direct RPCs, never
// delegated.
type GetPutReader struct {
	clnts clntAPI
	tgt   *Target

	off      sp.Toffset // start of the window
	splitEnd sp.Toffset // off + body: end of the split
	end      sp.Toffset // where tiling stops; splitEnd, or the end of a
	// delegated buffer which fell short of it (the file ended inside the window)
	max sp.Toffset // hard cap on the final chunk: splitEnd + slack, or, when
	// delegated, the end of the prefetched buffer
	probe sp.Tlength

	mu  sync.Mutex
	pos sp.Toffset // offset of the next chunk

	delegated bool
	buf       []byte // the prefetched window (delegated)
}

func NewGetPutReader(clnts *Clnts, pn string, off sp.Toffset, body sp.Tlength,
	slack, probe sp.Tlength, delegated bool, rpcIdx uint64) (*GetPutReader, error) {
	return newGetPutReader(clnts, pn, off, body, slack, probe, delegated, rpcIdx)
}

func newGetPutReader(clnts clntAPI, pn string, off sp.Toffset, body sp.Tlength,
	slack, probe sp.Tlength, delegated bool, rpcIdx uint64) (*GetPutReader, error) {
	tgt, err := ClassifyPath(pn)
	if err != nil {
		return nil, err
	}
	if probe == 0 {
		probe = minTailProbe
	}
	splitEnd := off + sp.Toffset(body)
	r := &GetPutReader{
		clnts:     clnts,
		tgt:       tgt,
		off:       off,
		splitEnd:  splitEnd,
		end:       splitEnd,
		max:       splitEnd + sp.Toffset(slack),
		probe:     probe,
		pos:       off,
		delegated: delegated,
	}
	if !delegated {
		// Chunks are fetched lazily, one per GetChunkReader call: no I/O here.
		return r, nil
	}
	// The one and only delegated get for this split: the cosandbox prefetched
	// exactly the (off, body+probe) window.
	b, err := clnts.delegatedGet(tgt, rpcIdx)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.MR, "GetPutReader %v off %v body %v prefetched %v", tgt, off, body, len(b))
	r.buf = b
	// A short fetch means the file ends within the window: nothing to extend.
	if uint64(len(b)) >= uint64(body)+uint64(probe) {
		if r.buf, err = r.extendTail(r.buf, off); err != nil {
			return nil, err
		}
	}
	// Tiles come out of the buffer, so it — not the split end plus slack — is
	// the hard cap, and a buffer which stopped short of the split end (EOF
	// inside the window) is where tiling stops.
	r.max = off + sp.Toffset(len(r.buf))
	if r.max < r.end {
		r.end = r.max
	}
	return r, nil
}

// GetChunkReader returns a reader for the next chunk of the window, the chunk's
// offset, and whether it is the window's final chunk. offinc lets the caller
// arrange overlap between consecutive chunks (the mapper overlaps by a word, so
// a word straddling a chunk boundary is still seen whole).
func (r *GetPutReader) GetChunkReader(sz, offinc int) (io.ReadCloser, sp.Toffset, bool, error) {
	r.mu.Lock()
	o, e, final, err := r.nextChunk(sz, offinc)
	buf := r.buf
	r.mu.Unlock()
	if err != nil {
		return nil, 0, false, err
	}
	if r.delegated {
		// Already in memory: the tile is a window into the prefetched buffer.
		// nextChunk capped e at the buffer's end, so the slice is in bounds.
		db.DPrintf(db.MR, "GetPutReader %v tile off %v len %v final %t", r.tgt, o, e-o, final)
		return io.NopCloser(bytes.NewReader(buf[o-r.off : e-r.off])), o, final, nil
	}
	b, err := r.fetchChunk(o, e, final)
	if err != nil {
		return nil, 0, false, err
	}
	db.DPrintf(db.MR, "GetPutReader %v chunk off %v len %v final %t", r.tgt, o, len(b), final)
	return io.NopCloser(bytes.NewReader(b)), o, final, nil
}

// nextChunk advances the window cursor, returning the next chunk's bounds. Held
// under the lock; the fetch itself is not, so that concurrent chunk readers
// overlap their gets.
func (r *GetPutReader) nextChunk(sz, offinc int) (o, e sp.Toffset, final bool, err error) {
	if r.pos >= r.end {
		return 0, 0, false, io.EOF
	}
	o = r.pos
	// The final chunk is the one containing the region's last byte. It runs to
	// the cap rather than to o+sz: it has to carry the bytes past the split end
	// that finish the straddling line. The direct path fetches those (fetchChunk
	// + extendTail); the delegated path already holds them, since the cap is the
	// end of the prefetched buffer.
	final = o+sp.Toffset(offinc) >= r.end
	if final {
		e = r.max
	} else {
		e = min(o+sp.Toffset(sz), r.max)
	}
	r.pos += sp.Toffset(offinc)
	return o, e, final, nil
}

// fetchChunk gets the bytes of one chunk. The final chunk is fetched through
// the split end plus a probe, and extended until the straddling line's newline.
func (r *GetPutReader) fetchChunk(o, e sp.Toffset, final bool) ([]byte, error) {
	if !final {
		return r.clnts.getChunk(r.tgt, uint64(o), uint64(e-o))
	}
	cnt := uint64(r.splitEnd-o) + uint64(r.probe)
	b, err := r.clnts.getChunk(r.tgt, uint64(o), cnt)
	if err != nil {
		return nil, err
	}
	// A short fetch means the file ends within the chunk: nothing to extend.
	if uint64(len(b)) < cnt {
		return b, nil
	}
	return r.extendTail(b, o)
}

// extendTail extends b — the bytes at [start, start+len(b)) — until it contains
// the first newline at or after splitEnd-1 (which terminates the line holding
// the split's last byte), the max cap, or EOF. All extension reads are direct
// RPCs, never delegated: the cosandbox deposited exactly one reply per rpcIdx.
func (r *GetPutReader) extendTail(b []byte, start sp.Toffset) ([]byte, error) {
	// Cap b at its length so that the appends below allocate a new array
	// instead of writing in place. A delegated reply's buffer points into the
	// shared-memory segment, and the segment's allocator hands out
	// segment[start:end] — a slice whose capacity runs to the *end of the
	// segment*. Appending onto that writes straight over whatever the allocator
	// handed out next, i.e. the frames of the delegated replies for this
	// mapper's later splits, which the cosandbox has already prefetched. Those
	// splits then fail to unmarshal ("cannot parse invalid wire-format data")
	// with perfectly valid offsets, far from here.
	b = b[:len(b):len(b)]
	scanFrom := 0
	if first := r.splitEnd - 1; first > start {
		scanFrom = int(first - start)
	}
	cnt := uint64(r.probe)
	for {
		if scanFrom < len(b) && bytes.IndexByte(b[scanFrom:], '\n') >= 0 {
			return b, nil
		}
		pos := start + sp.Toffset(len(b))
		if pos >= r.max {
			return b, nil
		}
		if rest := uint64(r.max - pos); cnt > rest {
			cnt = rest
		}
		nb, err := r.clnts.getChunk(r.tgt, uint64(pos), cnt)
		if err != nil {
			return nil, err
		}
		db.DPrintf(db.MR, "GetPutReader %v extend tail off %v cnt %v got %v", r.tgt, pos, cnt, len(nb))
		if len(nb) == 0 {
			// EOF
			return b, nil
		}
		b = append(b, nb...)
		if uint64(len(nb)) < cnt {
			// short read: EOF
			return b, nil
		}
		cnt *= 2
	}
}

func (r *GetPutReader) Close() error {
	return nil
}
