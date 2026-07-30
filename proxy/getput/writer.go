package getput

import (
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

// GetPutWriter is a ShardWriter that writes through the UX/S3 proxy Put
// RPCs. UX targets are written in CHUNK_SZ chunks at increasing offsets
// (the offset-0 chunk creates and truncates the file); S3 targets buffer
// the whole object and issue a single PutObject on Close, since S3 has no
// positional write (the S3 srv PutObject handler notes where
// multipart-upload support would slot in).
//
// Every put is a direct RPC. Delegation is deliberately input-only: a
// cosandbox prefetches a mapper's splits (see GetPutReader), but output never
// goes out through OutgoingDelegatedRPC.
type GetPutWriter struct {
	clnts   clntAPI
	tgt     *Target
	chunksz int
	buf     []byte
	off     uint64     // bytes flushed so far (UX)
	n       sp.Tlength // total bytes written
	created bool       // an offset-0 chunk has been issued (UX)
}

func NewGetPutWriter(clnts *Clnts, pn string) (*GetPutWriter, error) {
	return newGetPutWriter(clnts, pn)
}

func newGetPutWriter(clnts clntAPI, pn string) (*GetPutWriter, error) {
	tgt, err := ClassifyPath(pn)
	if err != nil {
		return nil, err
	}
	return &GetPutWriter{
		clnts:   clnts,
		tgt:     tgt,
		chunksz: int(sp.Conf.Chunk.CHUNK_SZ),
	}, nil
}

func (w *GetPutWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	w.n += sp.Tlength(len(p))
	if w.tgt.Kind == TUX {
		for len(w.buf) >= w.chunksz {
			if err := w.flushUX(w.buf[:w.chunksz]); err != nil {
				return 0, err
			}
			w.buf = w.buf[w.chunksz:]
		}
	}
	return len(p), nil
}

func (w *GetPutWriter) flushUX(b []byte) error {
	if err := w.clnts.putChunk(w.tgt, w.off, b); err != nil {
		return err
	}
	w.off += uint64(len(b))
	w.created = true
	return nil
}

func (w *GetPutWriter) Close() error {
	db.DPrintf(db.MR, "GetPutWriter close %v n %v", w.tgt, w.n)
	if w.tgt.Kind == TUX {
		// Flush the tail; always issue the offset-0 chunk so an empty
		// shard still creates the file.
		if len(w.buf) > 0 || !w.created {
			if err := w.flushUX(w.buf); err != nil {
				return err
			}
			w.buf = nil
		}
		return nil
	}
	if err := w.clnts.putObject(w.tgt, w.buf); err != nil {
		return err
	}
	w.buf = nil
	return nil
}

func (w *GetPutWriter) Nbytes() sp.Tlength {
	return w.n
}
