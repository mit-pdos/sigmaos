// Package getput provides a reader/writer layered above the UX and S3 proxy
// Get/Put RPC APIs, interface-compatible with the fslib streaming
// reader/writer the MR mapper uses, so the two can be swapped transparently
// (see claude-slop/MR_MAPPER_COSANDBOX.md). Reads may be served from the
// delegated-RPC store when a cosandbox prefetched them.
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
	Kind   Tkind
	Bucket string // S3
	Key    string // S3
	Path   string // UX: relative to the UX server's root
}

func (t *Target) String() string {
	if t.Kind == TS3 {
		return fmt.Sprintf("{s3 %v %v}", t.Bucket, t.Key)
	}
	return fmt.Sprintf("{ux %v}", t.Path)
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
		return &Target{Kind: TS3, Bucket: parts[1], Key: parts[2]}, nil
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
		return &Target{Kind: TUX, Path: parts[1]}, nil
	}
	return nil, fmt.Errorf("ClassifyPath: not a ux or s3 path %q", pn)
}

// Clnts lazily creates and caches the UX and S3 proxy clients for the local
// kernel.
type Clnts struct {
	fsl *fslib.FsLib

	mu  sync.Mutex
	uxc *uxclnt.UXClnt
	s3c *s3clnt.S3Clnt
}

func NewClnts(fsl *fslib.FsLib) *Clnts {
	return &Clnts{fsl: fsl}
}

func (c *Clnts) UX() (*uxclnt.UXClnt, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.uxc == nil {
		start := time.Now()
		pn := filepath.Join(sp.UX, c.fsl.ProcEnv().GetKernelID())
		uxc, err := uxclnt.NewUXClnt(c.fsl, pn)
		if err != nil {
			db.DPrintf(db.ERROR, "Err NewUXClnt %v: %v", pn, err)
			return nil, err
		}
		c.uxc = uxc
		pe := c.fsl.ProcEnv()
		perf.LogSpawnLatency("getput.Clnts.UX", pe.GetPID(), pe.GetSpawnTime(), start)
	}
	return c.uxc, nil
}

func (c *Clnts) S3() (*s3clnt.S3Clnt, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.s3c == nil {
		pn := filepath.Join(sp.S3, c.fsl.ProcEnv().GetKernelID())
		s3c, err := s3clnt.NewS3Clnt(c.fsl, pn)
		if err != nil {
			db.DPrintf(db.ERROR, "Err NewS3Clnt %v: %v", pn, err)
			return nil, err
		}
		c.s3c = s3c
	}
	return c.s3c, nil
}

// clntAPI is the internal surface GetPutReader/GetPutWriter use, so tests
// can substitute an in-memory implementation for *Clnts.
type clntAPI interface {
	getChunk(tgt *Target, off, cnt uint64) ([]byte, error)
	delegatedGet(tgt *Target, rpcIdx uint64) ([]byte, error)
	putChunk(tgt *Target, off uint64, b []byte) error // UX targets only
	putObject(tgt *Target, b []byte) error            // S3 targets only
}

// getChunk issues a direct (non-delegated) ranged get for the target.
func (c *Clnts) getChunk(tgt *Target, off, cnt uint64) ([]byte, error) {
	if tgt.Kind == TUX {
		uxc, err := c.UX()
		if err != nil {
			return nil, err
		}
		return uxc.GetFileChunk(tgt.Path, off, cnt)
	}
	s3c, err := c.S3()
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
		uxc, err := c.UX()
		if err != nil {
			return nil, err
		}
		b, _, err := uxc.DelegatedGetFile(rpcIdx)
		return b, err
	}
	s3c, err := c.S3()
	if err != nil {
		return nil, err
	}
	b, _, err := s3c.DelegatedGetObject(rpcIdx)
	return b, err
}

// putChunk writes b at byte offset off on a UX target (the offset-0 chunk
// creates and truncates the file).
func (c *Clnts) putChunk(tgt *Target, off uint64, b []byte) error {
	uxc, err := c.UX()
	if err != nil {
		return err
	}
	return uxc.PutFileChunk(tgt.Path, off, b)
}

// putObject writes a whole object on an S3 target.
func (c *Clnts) putObject(tgt *Target, b []byte) error {
	s3c, err := c.S3()
	if err != nil {
		return err
	}
	return s3c.PutObject(tgt.Bucket, tgt.Key, b)
}
