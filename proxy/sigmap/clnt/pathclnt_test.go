package clnt

// Tests that an SPProxyClnt serves a mounted path client's paths in-proc rather
// than sending them to spproxyd. The clnt is built by hand with no connection
// to spproxyd at all: a test which accidentally routed one of these operations
// over the proxy would nil-panic on scc.rpcc, which is exactly the property we
// want.

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	sos "sigmaos/api/sigmaos"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

// A PathClntAPI which records what it is asked to do and serves reads/writes
// out of one in-memory object.
type fakePathClnt struct {
	created []string
	opened  []string
	data    []byte
	clunked []sp.Tfid
	nextFid sp.Tfid
}

func (f *fakePathClnt) Create(pn sp.Tsigmapath, _ *sp.Tprincipal, _ sp.Tperm, _ sp.Tmode, _ sp.TleaseId, _ *sp.Tfence) (sp.Tfid, error) {
	f.created = append(f.created, pn)
	f.nextFid++
	return f.nextFid, nil
}

func (f *fakePathClnt) Open(pn sp.Tsigmapath, _ *sp.Tprincipal, _ sp.Tmode, _ sos.Watch) (sp.Tfid, error) {
	f.opened = append(f.opened, pn)
	f.nextFid++
	return f.nextFid, nil
}

func (f *fakePathClnt) WriteF(_ sp.Tfid, off sp.Toffset, b []byte, _ *sp.Tfence) (sp.Tsize, error) {
	if int(off) > len(f.data) {
		f.data = append(f.data, make([]byte, int(off)-len(f.data))...)
	}
	f.data = append(f.data[:off], b...)
	return sp.Tsize(len(b)), nil
}

func (f *fakePathClnt) ReadF(_ sp.Tfid, off sp.Toffset, b []byte, _ *sp.Tfence) (sp.Tsize, error) {
	if int(off) >= len(f.data) {
		return 0, nil
	}
	n := copy(b, f.data[off:])
	return sp.Tsize(n), nil
}

func (f *fakePathClnt) PreadRdr(_ sp.Tfid, _ sp.Toffset, _ sp.Tsize) (io.ReadCloser, error) {
	return nil, nil
}

func (f *fakePathClnt) Clunk(fid sp.Tfid) error {
	f.clunked = append(f.clunked, fid)
	return nil
}

func newTestClnt() *SPProxyClnt {
	pe := proc.NewTestProcEnv(sp.ROOTREALM, nil, nil, "10.0.0.1", "10.0.0.1", "", true, false)
	return &SPProxyClnt{pe: pe, pcs: newPathClntTable()}
}

func TestMountedPathIsServedLocally(t *testing.T) {
	scc := newTestClnt()
	f := &fakePathClnt{}
	assert.Nil(t, scc.MountPathClnt(sp.S3CLNT, f))

	pn := sp.S3CLNT + "/bkt/dir/obj"
	fd, err := scc.Create(pn, 0777, sp.OWRITE)
	assert.Nil(t, err)
	assert.True(t, isLocalFd(fd), "fd %v should be local", fd)
	assert.Equal(t, []string{pn}, f.created)

	// Sequential writes must advance the offset, since PathClntAPI is
	// offset-based and the caller (a buffered writer) does not pass one.
	n, err := scc.Write(fd, []byte("hello "))
	assert.Nil(t, err)
	assert.Equal(t, sp.Tsize(6), n)
	_, err = scc.Write(fd, []byte("world"))
	assert.Nil(t, err)
	assert.Equal(t, "hello world", string(f.data))

	assert.Nil(t, scc.CloseFd(fd))
	assert.Equal(t, 1, len(f.clunked))
	// The fd is gone, so a use-after-close is an error rather than a stray
	// operation against a recycled fd.
	_, err = scc.Write(fd, []byte("x"))
	assert.NotNil(t, err)
}

func TestMountedPathReadsTrackOffset(t *testing.T) {
	scc := newTestClnt()
	f := &fakePathClnt{data: []byte("0123456789")}
	assert.Nil(t, scc.MountPathClnt(sp.S3CLNT, f))

	fd, err := scc.Open(sp.S3CLNT+"/bkt/obj", sp.OREAD, sos.O_NOW)
	assert.Nil(t, err)

	b := make([]byte, 4)
	n, err := scc.Read(fd, b)
	assert.Nil(t, err)
	assert.Equal(t, "0123", string(b[:n]))
	n, err = scc.Read(fd, b)
	assert.Nil(t, err)
	assert.Equal(t, "4567", string(b[:n]))

	// Pread must not disturb the sequential offset.
	n, err = scc.Pread(fd, b, 0)
	assert.Nil(t, err)
	assert.Equal(t, "0123", string(b[:n]))
	n, err = scc.Read(fd, b)
	assert.Nil(t, err)
	assert.Equal(t, "89", string(b[:n]))

	// Seek repositions it.
	assert.Nil(t, scc.Seek(fd, 2))
	n, err = scc.Read(fd, b)
	assert.Nil(t, err)
	assert.Equal(t, "2345", string(b[:n]))
}

func TestUnmountedPathIsNotLocal(t *testing.T) {
	scc := newTestClnt()
	assert.Nil(t, scc.MountPathClnt(sp.S3CLNT, &fakePathClnt{}))

	// Only the first path component decides, and only for a mounted one: an
	// ordinary sigma pathname is left to spproxyd.
	for _, pn := range []string{"name/s3/~local/bkt/obj", "name/ux/~local/f", "s3clntx/bkt/obj"} {
		_, ok := scc.pcs.lookup(pn)
		assert.False(t, ok, "%v should not resolve to a local path client", pn)
	}
	_, ok := scc.pcs.lookup(sp.S3CLNT + "/bkt/obj")
	assert.True(t, ok)
}

func TestLocalFdsDontCollideWithProxyFds(t *testing.T) {
	scc := newTestClnt()
	assert.Nil(t, scc.MountPathClnt(sp.S3CLNT, &fakePathClnt{}))

	// spproxyd hands out small fds; locally-served ones must never land in that
	// range, since an fd is routed by value alone.
	for i := 0; i < 64; i++ {
		fd, err := scc.Create(sp.S3CLNT+"/bkt/obj", 0777, sp.OWRITE)
		assert.Nil(t, err)
		assert.True(t, fd >= localFdBase, "local fd %v collides with proxy fd space", fd)
	}
	assert.False(t, isLocalFd(0))
	assert.False(t, isLocalFd(3))
}
