// Package getput provides a reader/writer layered above the UX and S3 proxy
// Get/Put RPC APIs, interface-compatible with the fslib streaming
// reader/writer the MR mapper uses, so the two can be swapped transparently.
// Reads may be served from the delegated-RPC store when a cosandbox prefetched
// them.
package getput

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	db "sigmaos/debug"
	s3clnt "sigmaos/proxy/s3/clnt"
	uxclnt "sigmaos/proxy/ux/clnt"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

// SplitReader is the input-side surface Mapper.doSplit needs.
// *fslib.ParallelFileReader satisfies it (the bool is the final-window
// flag).
type SplitReader interface {
	GetChunkReader(sz, offinc int) (io.ReadCloser, sp.Toffset, bool, error)
	Close() error
}

// ShardWriter is the output-side surface Mapper.initWrt/Emit/closewrts
// need. *fslib.FileWriter satisfies it.
type ShardWriter interface {
	io.Writer
	Close() error
	Nbytes() sp.Tlength
}

type Tkind int

const (
	TUX Tkind = iota
	TS3
)

// Target is the classification of a sigma pathname into an UX or S3 proxy
// RPC target.
type Target struct {
	Kind Tkind
	// Which server's proxy serves this target: the kernel ID from the
	// pathname, or "" for a path that doesn't name one (the s3clnt form, or a
	// union element like ~local). Clnts treats both as the local proxy. A
	// mapper only ever reads through its local proxy; a reducer reads one
	// mapper shard per kernel, so its targets do name servers.
	Kid    string
	Bucket string // S3
	Key    string // S3
	Path   string // UX: relative to the UX server's root
}

func (t *Target) String() string {
	if t.Kind == TS3 {
		return fmt.Sprintf("{s3 %v %v %v}", t.Kid, t.Bucket, t.Key)
	}
	return fmt.Sprintf("{ux %v %v}", t.Kid, t.Path)
}

// isLocal reports whether this target is served by the local proxy: either the
// pathname didn't name a server, or it named one with a union element.
func (t *Target) isLocal() bool {
	return t.Kid == "" || strings.HasPrefix(t.Kid, "~")
}

// ClassifyPath classifies a pathname as an UX or S3 proxy target. It
// accepts name/s3/<kid>/<bucket>/<key...>, the s3clnt/<bucket>/<key...>
// form produced by sp.S3ClientPath, and name/ux/<kid>/<path...> (the UX RPC
// path is relative to the UX server's root, the same convention the etcd
// and memcached cosandbox jobs use).
func ClassifyPath(pn string) (*Target, error) {
	if rest, ok := strings.CutPrefix(pn, sp.S3); ok {
		parts := strings.SplitN(rest, "/", 3) // kid, bucket, key
		if len(parts) < 3 || parts[1] == "" || parts[2] == "" {
			return nil, fmt.Errorf("ClassifyPath: malformed s3 path %q", pn)
		}
		return &Target{Kind: TS3, Kid: parts[0], Bucket: parts[1], Key: parts[2]}, nil
	}
	if rest, ok := strings.CutPrefix(pn, sp.S3CLNT+"/"); ok {
		parts := strings.SplitN(rest, "/", 2) // bucket, key
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("ClassifyPath: malformed s3clnt path %q", pn)
		}
		return &Target{Kind: TS3, Bucket: parts[0], Key: parts[1]}, nil
	}
	if rest, ok := strings.CutPrefix(pn, sp.UX); ok {
		parts := strings.SplitN(rest, "/", 2) // kid, path
		if len(parts) < 2 || parts[1] == "" {
			return nil, fmt.Errorf("ClassifyPath: malformed ux path %q", pn)
		}
		return &Target{Kind: TUX, Kid: parts[0], Path: parts[1]}, nil
	}
	return nil, fmt.Errorf("ClassifyPath: not a ux or s3 path %q", pn)
}

// Clnts lazily creates and caches UX and S3 proxy clients, keyed by kernel
// ID. A mapper only ever talks to its local proxies, so it ends up with one of
// each; a reducer reads one shard per mapper and so holds a UX client per
// kernel whose shards it reads.
type Clnts struct {
	fsl *fslib.FsLib

	mu   sync.Mutex
	uxcs map[string]*uxclnt.UXClnt
	s3cs map[string]*s3clnt.S3Clnt
}

func NewClnts(fsl *fslib.FsLib) *Clnts {
	return &Clnts{
		fsl:  fsl,
		uxcs: make(map[string]*uxclnt.UXClnt),
		s3cs: make(map[string]*s3clnt.S3Clnt),
	}
}

// kernel resolves the kernel whose proxy serves tgt, mapping a target that
// doesn't name one to this proc's own kernel.
func (c *Clnts) kernel(tgt *Target) string {
	if tgt.isLocal() {
		return c.fsl.ProcEnv().GetKernelID()
	}
	return tgt.Kid
}

func (c *Clnts) UX(kid string) (*uxclnt.UXClnt, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if uxc, ok := c.uxcs[kid]; ok {
		return uxc, nil
	}
	start := time.Now()
	pn := filepath.Join(sp.UX, kid)
	uxc, err := uxclnt.NewUXClnt(c.fsl, pn)
	if err != nil {
		db.DPrintf(db.ERROR, "Err NewUXClnt %v: %v", pn, err)
		return nil, err
	}
	c.uxcs[kid] = uxc
	pe := c.fsl.ProcEnv()
	perf.LogSpawnLatency("getput.Clnts.UX."+kid, pe.GetPID(), pe.GetSpawnTime(), start)
	return uxc, nil
}

func (c *Clnts) S3(kid string) (*s3clnt.S3Clnt, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s3c, ok := c.s3cs[kid]; ok {
		return s3c, nil
	}
	pn := filepath.Join(sp.S3, kid)
	s3c, err := s3clnt.NewS3Clnt(c.fsl, pn)
	if err != nil {
		db.DPrintf(db.ERROR, "Err NewS3Clnt %v: %v", pn, err)
		return nil, err
	}
	c.s3cs[kid] = s3c
	return s3c, nil
}

// clntAPI is the internal surface GetPutReader/GetPutWriter use, so tests
// can substitute an in-memory implementation for *Clnts.
type clntAPI interface {
	getChunk(tgt *Target, off, cnt uint64) ([]byte, error)
	delegatedGet(tgt *Target, rpcIdx uint64) ([]byte, error)
	putChunk(tgt *Target, off uint64, b []byte) error // UX targets only
	putObject(tgt *Target, b []byte) error            // S3 targets only
}

// getChunk issues a direct (non-delegated) ranged get for the target. A count
// of 0 means the whole file/object (both proxies read to EOF).
func (c *Clnts) getChunk(tgt *Target, off, cnt uint64) ([]byte, error) {
	if tgt.Kind == TUX {
		uxc, err := c.UX(c.kernel(tgt))
		if err != nil {
			return nil, err
		}
		return uxc.GetFileChunk(tgt.Path, off, cnt)
	}
	s3c, err := c.S3(c.kernel(tgt))
	if err != nil {
		return nil, err
	}
	return s3c.GetObjectChunk(tgt.Bucket, tgt.Key, off, cnt)
}

// delegatedGet retrieves the reply the cosandbox deposited for rpcIdx. Each
// rpcIdx has exactly one reply; callers must never issue a second delegated
// get for the same split — any follow-up reads must be direct RPCs
// (getChunk).
func (c *Clnts) delegatedGet(tgt *Target, rpcIdx uint64) ([]byte, error) {
	if tgt.Kind == TUX {
		uxc, err := c.UX(c.kernel(tgt))
		if err != nil {
			return nil, err
		}
		b, _, err := uxc.DelegatedGetFile(rpcIdx)
		return b, err
	}
	s3c, err := c.S3(c.kernel(tgt))
	if err != nil {
		return nil, err
	}
	b, _, err := s3c.DelegatedGetObject(rpcIdx)
	return b, err
}

// putChunk writes b at byte offset off on a UX target (the offset-0 chunk
// creates and truncates the file).
func (c *Clnts) putChunk(tgt *Target, off uint64, b []byte) error {
	uxc, err := c.UX(c.kernel(tgt))
	if err != nil {
		return err
	}
	return uxc.PutFileChunk(tgt.Path, off, b)
}

// putObject writes a whole object on an S3 target.
func (c *Clnts) putObject(tgt *Target, b []byte) error {
	s3c, err := c.S3(c.kernel(tgt))
	if err != nil {
		return err
	}
	return s3c.PutObject(tgt.Bucket, tgt.Key, b)
}
