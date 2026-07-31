package getput

import (
	"bytes"
	"io"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

// FileReader is the whole-file read surface Reducer.readFile needs.
// *fslib.BufReader satisfies it too, so the two can be swapped.
type FileReader interface {
	io.ReadCloser
	Nbytes() sp.Tlength
}

// GetPutFileReader fetches a whole file or object in one get — a direct RPC to
// the proxy serving it, or a delegated get of what a cosandbox prefetched —
// and serves it from memory. Unlike GetPutReader, which fetches a window of a
// split and hunts for line boundaries, this reads to EOF and interprets
// nothing: a mapper's output shard is a stream of encoded KVs, and a reducer
// consumes all of it.
type GetPutFileReader struct {
	rdr *bytes.Reader
	n   sp.Tlength
}

func NewGetPutFileReader(clnts *Clnts, pn string, delegated bool, rpcIdx uint64) (*GetPutFileReader, error) {
	return newGetPutFileReader(clnts, pn, delegated, rpcIdx)
}

func newGetPutFileReader(clnts clntAPI, pn string, delegated bool, rpcIdx uint64) (*GetPutFileReader, error) {
	tgt, err := ClassifyPath(pn)
	if err != nil {
		return nil, err
	}
	var b []byte
	if delegated {
		// The one and only delegated get for this file: the cosandbox
		// prefetched all of it.
		b, err = clnts.delegatedGet(tgt, rpcIdx)
	} else {
		// Count 0 is "to EOF" at both proxies.
		b, err = clnts.getChunk(tgt, 0, 0)
	}
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.MR, "GetPutFileReader %v fetched %v delegated %t", tgt, len(b), delegated)
	return &GetPutFileReader{rdr: bytes.NewReader(b), n: sp.Tlength(len(b))}, nil
}

func (r *GetPutFileReader) Read(p []byte) (int, error) {
	return r.rdr.Read(p)
}

func (r *GetPutFileReader) Close() error {
	return nil
}

// Nbytes reports the size of the file fetched, which is what the reducer
// accounts as bytes read (it consumes all of it).
func (r *GetPutFileReader) Nbytes() sp.Tlength {
	return r.n
}
