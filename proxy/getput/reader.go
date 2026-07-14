package getput

import (
	"bytes"
	"io"
	"sync"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

// GetPutReader is a SplitReader that fetches a split's read window
// ([off, off+body) plus a tail probe) into memory in one get — a direct
// ranged RPC to the local proxy, or a delegated get of the bytes a
// cosandbox prefetched — and serves it as a single final chunk.
// DoChunk's final-chunk path processes it line-granularly to the split end,
// so surplus probe bytes need no trimming. If the line straddling the split
// end doesn't terminate within the probe, the reader lazily extends the
// buffer with direct (never delegated: the cosandbox deposited exactly one
// reply for this split's rpcIdx) ranged gets, doubling and capped at
// body+slack past off.
type GetPutReader struct {
	mu     sync.Mutex
	served bool

	buf []byte     // window body + tail bytes fetched so far
	off sp.Toffset // absolute offset of buf[0]
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
	splitEnd := off + sp.Toffset(body)
	max := splitEnd + sp.Toffset(slack)
	fetch := uint64(body) + uint64(probe)
	var b []byte
	if delegated {
		// The one and only delegated get for this split: the cosandbox
		// prefetched exactly the (off, body+probe) window.
		b, err = clnts.delegatedGet(tgt, rpcIdx)
	} else {
		b, err = clnts.getChunk(tgt, uint64(off), fetch)
	}
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.MR, "GetPutReader %v off %v body %v fetched %v delegated %t", tgt, off, body, len(b), delegated)
	r := &GetPutReader{
		buf: b,
		off: off,
	}
	// A short fetch means the file ends within the window: nothing to
	// extend. Otherwise make sure the buffer reaches the end of the line
	// containing the split's last byte.
	if uint64(len(b)) >= fetch {
		if err := r.extendTail(clnts, tgt, splitEnd, max, probe); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// extendTail extends buf until it contains the first newline at or after
// splitEnd-1 (which terminates the line containing the split's last byte),
// the max cap, or EOF. All extension reads are direct RPCs.
func (r *GetPutReader) extendTail(clnts clntAPI, tgt *Target, splitEnd, max sp.Toffset, probe sp.Tlength) error {
	scanFrom := 0
	if first := splitEnd - 1; first > r.off {
		scanFrom = int(first - r.off)
	}
	cnt := uint64(probe)
	for {
		if scanFrom < len(r.buf) && bytes.IndexByte(r.buf[scanFrom:], '\n') >= 0 {
			return nil
		}
		pos := r.off + sp.Toffset(len(r.buf))
		if pos >= max {
			return nil
		}
		if rest := uint64(max - pos); cnt > rest {
			cnt = rest
		}
		b, err := clnts.getChunk(tgt, uint64(pos), cnt)
		if err != nil {
			return err
		}
		db.DPrintf(db.MR, "GetPutReader %v extend tail off %v cnt %v got %v", tgt, pos, cnt, len(b))
		if len(b) == 0 {
			// EOF
			return nil
		}
		r.buf = append(r.buf, b...)
		if uint64(len(b)) < cnt {
			// short read: EOF
			return nil
		}
		cnt *= 2
	}
}

// GetChunkReader serves the whole window as a single final chunk (the
// sz/offinc window-tiling params are ignored); subsequent calls return
// io.EOF.
func (r *GetPutReader) GetChunkReader(sz, offinc int) (io.ReadCloser, sp.Toffset, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.served {
		return nil, 0, false, io.EOF
	}
	r.served = true
	return io.NopCloser(bytes.NewReader(r.buf)), r.off, true, nil
}

func (r *GetPutReader) Close() error {
	return nil
}
