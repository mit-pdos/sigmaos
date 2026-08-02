package clnt

// Local path clients for a proc which reaches SigmaOS through spproxy.
//
// Such a proc has no namespace of its own: every path operation is an RPC to
// spproxyd. A path client mounted with MountPathClnt is the exception. It *is*
// local code — the S3 path client, say, which talks to S3 directly with the AWS
// SDK — and forwarding its pathnames to spproxyd could only fail, since
// spproxyd has no such mount and its FsClient resolves the first path component
// against its own mount table ("file not found: s3clnt").
//
// So keep a small mount table and fd table here and serve those paths in-proc,
// mirroring what sigmaclnt/fsclnt does for a proc which owns its namespace.

import (
	"sync"

	sos "sigmaos/api/sigmaos"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

// Locally-served fds are numbered from here up, so that an fd can be routed by
// value alone and can never be confused with one spproxyd handed out.
const localFdBase = 1 << 20

type localFd struct {
	fid  sp.Tfid
	pc   sos.PathClntAPI
	mode sp.Tmode
	off  sp.Toffset
	pn   sp.Tsigmapath
}

type pathClntTable struct {
	mu     sync.Mutex
	mnts   map[string]sos.PathClntAPI
	fds    map[int]*localFd
	nextFd int
}

func newPathClntTable() *pathClntTable {
	return &pathClntTable{
		mnts:   make(map[string]sos.PathClntAPI),
		fds:    make(map[int]*localFd),
		nextFd: localFdBase,
	}
}

func (t *pathClntTable) mount(mnt sp.Tsigmapath, pc sos.PathClntAPI) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.mnts[mnt] = pc
}

// lookup returns the path client serving pn, if any. Like fsclnt's mntLookup it
// matches on the first path component only.
func (t *pathClntTable) lookup(pn sp.Tsigmapath) (sos.PathClntAPI, bool) {
	p := path.Split(pn)
	if len(p) == 0 {
		return nil, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.mnts) == 0 {
		return nil, false
	}
	pc, ok := t.mnts[p[0]]
	return pc, ok
}

func (t *pathClntTable) allocFd(fid sp.Tfid, pc sos.PathClntAPI, mode sp.Tmode, pn sp.Tsigmapath) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	fd := t.nextFd
	t.nextFd++
	t.fds[fd] = &localFd{fid: fid, pc: pc, mode: mode, pn: pn}
	return fd
}

// isLocal reports whether fd is served by a local path client. Cheap enough to
// call on every fd operation.
func isLocalFd(fd int) bool {
	return fd >= localFdBase
}

func (t *pathClntTable) lookupFd(fd int) (*localFd, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	f, ok := t.fds[fd]
	if !ok {
		return nil, serr.NewErr(serr.TErrNotfound, fd)
	}
	return f, nil
}

func (t *pathClntTable) freeFd(fd int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.fds, fd)
}

// The offset of a sequential read/write is tracked here, since PathClntAPI's
// ReadF/WriteF are offset-based.
func (t *pathClntTable) off(fd int) (sp.Toffset, error) {
	f, err := t.lookupFd(fd)
	if err != nil {
		return 0, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	return f.off, nil
}

func (t *pathClntTable) incOff(fd int, n sp.Toffset) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if f, ok := t.fds[fd]; ok {
		f.off += n
	}
}

func (t *pathClntTable) setOff(fd int, o sp.Toffset) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	f, ok := t.fds[fd]
	if !ok {
		return serr.NewErr(serr.TErrNotfound, fd)
	}
	f.off = o
	return nil
}
