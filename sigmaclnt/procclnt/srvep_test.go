package procclnt_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt/procclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

// Tests for the explicit server-endpoint caching helpers. The parent half
// (SrvEPCache) is exercised directly; for the child half these tests play the
// child themselves, by stamping endpoints into their own ProcEnv the way a
// parent's Spawn would.

// A ~local pathname — what a cosandbox manifest names, since it is built
// before the proc is placed — resolves to the entry the parent cached for the
// kernel the proc landed on. Needs no kernel.
func TestLookupCachedEndpointLocal(t *testing.T) {
	const kid = "kernel-1"
	pe := proc.NewProcEnvUnset(false)
	pe.KernelID = kid
	ep := sp.NewEndpoint(sp.INTERNAL_EP, sp.Taddrs{sp.NewTaddr(sp.Tip("10.0.0.1"), sp.Tport(1111))})
	pe.SetCachedEndpoint(filepath.Join(sp.UX, kid), ep)

	for _, pn := range []string{filepath.Join(sp.UX, kid), filepath.Join(sp.UX, sp.LOCAL)} {
		got, ok := procclnt.LookupCachedEndpoint(pe, pn)
		if assert.True(t, ok, "no cached EP for %v", pn) {
			assert.Equal(t, ep.String(), got.String(), "wrong EP for %v", pn)
		}
	}

	// Another kernel's server, a service we cached nothing for, ~any (which
	// isn't ~local), and a name that merely starts with ~local: all misses.
	for _, pn := range []string{
		filepath.Join(sp.UX, "kernel-2"),
		filepath.Join(sp.S3, kid),
		filepath.Join(sp.UX, sp.ANY),
		filepath.Join(sp.UX, sp.LOCAL+"dir"),
	} {
		_, ok := procclnt.LookupCachedEndpoint(pe, pn)
		assert.False(t, ok, "unexpected cached EP for %v", pn)
	}

	// With no kernel ID there is no local server to resolve to.
	pe.KernelID = sp.NOT_SET
	_, ok := procclnt.LookupCachedEndpoint(pe, filepath.Join(sp.UX, sp.LOCAL))
	assert.False(t, ok, "resolved ~local without a kernel ID")
}

func uxSrvs(ts *test.Tstate) []string {
	sts, err := ts.GetDir(sp.UX)
	assert.Nil(ts.T, err, "GetDir %v: %v", sp.UX, err)
	return sp.Names(sts)
}

// A parent discovers every UX server's endpoint and stamps them onto a proc.
func TestSrvEPCacheDiscoverAndCache(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	srvs := uxSrvs(ts)
	if !assert.True(t, len(srvs) > 0, "no UX servers") {
		return
	}

	epc := procclnt.NewSrvEPCache(ts.FsLib, sp.UX)
	eps, err := epc.Endpoints()
	assert.Nil(t, err, "Endpoints err %v", err)
	assert.Equal(t, len(srvs), len(eps), "discovered %v, want %v servers", eps, srvs)
	for _, srv := range srvs {
		pn := filepath.Join(sp.UX, srv)
		_, ok := eps[pn]
		assert.True(t, ok, "no EP for %v in %v", pn, eps)
	}

	// A second call is served from the cache: same map, no re-discovery.
	eps2, err := epc.Endpoints()
	assert.Nil(t, err, "Endpoints (cached) err %v", err)
	assert.Equal(t, len(eps), len(eps2), "cached EPs differ")

	// Stamping puts them where the child looks for them.
	p := proc.NewProc("sleeper", []string{"1ms", ""})
	assert.Nil(t, epc.CacheEndpoints(p), "CacheEndpoints")
	for _, srv := range srvs {
		pn := filepath.Join(sp.UX, srv)
		ep, ok := p.GetProcEnv().GetCachedEndpoint(pn)
		assert.True(t, ok, "proc has no cached EP for %v", pn)
		assert.Equal(t, eps[pn].String(), ep.String(), "EP for %v differs", pn)
	}

	// Refresh() replaces the endpoints without disturbing readers.
	epc.Refresh()
	eps3, err := epc.Endpoints()
	assert.Nil(t, err, "Endpoints (post-refresh) err %v", err)
	assert.Equal(t, len(eps), len(eps3), "post-refresh EPs differ")
}

// A child mounts a UX server it was given the endpoint for, and reads/writes
// through the mount.
func TestMountCachedEndpoint(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	srvs := uxSrvs(ts)
	if !assert.True(t, len(srvs) > 0, "no UX servers") {
		return
	}
	pn := filepath.Join(sp.UX, srvs[0])

	// No EP cached for pn yet: the helper reports that and does nothing.
	ok, err := procclnt.MountCachedEndpoint(ts.FsLib, pn)
	assert.Nil(t, err, "MountCachedEndpoint (uncached) err %v", err)
	assert.False(t, ok, "reported an EP for %v with none cached", pn)

	// Play the parent: stamp the EP where the child half reads it from.
	ep, err := ts.ReadEndpoint(pn)
	if !assert.Nil(t, err, "ReadEndpoint %v err %v", pn, err) {
		return
	}
	ts.ProcEnv().SetCachedEndpoint(pn, ep)

	ok, err = procclnt.MountCachedEndpoint(ts.FsLib, pn)
	assert.Nil(t, err, "MountCachedEndpoint err %v", err)
	assert.True(t, ok, "no EP for %v", pn)
	assert.True(t, mounted(ts, pn), "%v not mounted: %v", pn, ts.Mounts())

	// Idempotent.
	ok, err = procclnt.MountCachedEndpoint(ts.FsLib, pn)
	assert.Nil(t, err, "MountCachedEndpoint (again) err %v", err)
	assert.True(t, ok, "no EP for %v on second mount", pn)

	// The mount works: write and read a file under it.
	fn := filepath.Join(pn, "srvep-test-file")
	ts.Remove(fn)
	_, err = ts.PutFile(fn, 0777, sp.OWRITE, []byte("cached-ep"))
	assert.Nil(t, err, "PutFile %v err %v", fn, err)
	b, err := ts.GetFile(fn)
	assert.Nil(t, err, "GetFile %v err %v", fn, err)
	assert.Equal(t, "cached-ep", string(b), "contents")
	ts.Remove(fn)
}

// A child mounts its local UX server under both <unionpn>/<kid> and
// <unionpn>/~local, and a walk of the ~local spelling is then served by the
// mount table instead of scanning the union directory in named.
func TestMountCachedLocalSrv(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	srvs := uxSrvs(ts)
	if !assert.True(t, len(srvs) > 0, "no UX servers") {
		return
	}

	// A test proc has no kernel ID, so there is no local server to mount.
	ok, err := procclnt.MountCachedLocalSrv(ts.FsLib, sp.UX)
	assert.Nil(t, err, "MountCachedLocalSrv (no kernel ID) err %v", err)
	assert.False(t, ok, "mounted a local srv without a kernel ID")

	// Play a proc running on the kernel that hosts srvs[0]: give ourselves
	// its kernel ID and the endpoint its parent would have cached. The
	// Tstate's ProcEnv is shared with later tests, so put it back after.
	kid := srvs[0]
	pn := filepath.Join(sp.UX, kid)
	localpn := filepath.Join(sp.UX, sp.LOCAL)
	ep, err := ts.ReadEndpoint(pn)
	if !assert.Nil(t, err, "ReadEndpoint %v err %v", pn, err) {
		return
	}
	pe := ts.ProcEnv()
	savedKid := pe.GetKernelID()
	pe.KernelID = kid
	pe.SetCachedEndpoint(pn, ep)
	defer func() {
		pe.KernelID = savedKid
		pe.ClearCachedEndpoint(pn)
	}()

	ok, err = procclnt.MountCachedLocalSrv(ts.FsLib, sp.UX)
	assert.Nil(t, err, "MountCachedLocalSrv err %v", err)
	assert.True(t, ok, "no EP for local srv %v", pn)
	assert.True(t, mounted(ts, pn), "%v not mounted: %v", pn, ts.Mounts())
	assert.True(t, mounted(ts, localpn), "%v not mounted: %v", localpn, ts.Mounts())

	// Walking ~local now resolves out of the mount table: no union scan, and
	// so no endpoint-file read from named.
	before := unionScans(ts)
	fn := filepath.Join(localpn, "srvep-local-test-file")
	ts.Remove(fn)
	_, err = ts.PutFile(fn, 0777, sp.OWRITE, []byte("local"))
	assert.Nil(t, err, "PutFile %v err %v", fn, err)
	b, err := ts.GetFile(fn)
	assert.Nil(t, err, "GetFile %v err %v", fn, err)
	assert.Equal(t, "local", string(b), "contents")
	assert.Equal(t, before, unionScans(ts), "walking %v scanned the union dir", localpn)

	// The file is the same one the concrete path names.
	b, err = ts.GetFile(filepath.Join(pn, "srvep-local-test-file"))
	assert.Nil(t, err, "GetFile via %v err %v", pn, err)
	assert.Equal(t, "local", string(b), "contents via concrete path")
	ts.Remove(fn)
}

// A stale endpoint must not wedge the path: the mount fails (or the server it
// names is simply gone) and walking finds the server through named as before.
func TestMountCachedEndpointStale(t *testing.T) {
	ts, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer ts.Shutdown()

	srvs := uxSrvs(ts)
	if !assert.True(t, len(srvs) > 0, "no UX servers") {
		return
	}
	pn := filepath.Join(sp.UX, srvs[0])

	// An endpoint pointing at a port nothing listens on: attaching should
	// fail fast (connection refused) rather than hang.
	bogus := sp.NewEndpoint(sp.INTERNAL_EP, sp.Taddrs{sp.NewTaddr(sp.Tip("127.0.0.1"), sp.Tport(1))})
	ts.ProcEnv().SetCachedEndpoint(pn, bogus)
	defer ts.ProcEnv().ClearCachedEndpoint(pn)

	ok, err := procclnt.MountCachedEndpoint(ts.FsLib, pn)
	assert.True(t, ok, "reported no EP for %v", pn)
	db.DPrintf(db.TEST, "MountCachedEndpoint with stale EP: err %v", err)

	// Whether the attach failed or not, the path still works: MountTree
	// removes a mount point it couldn't attach, so this walks through named.
	_, err = ts.GetDir(pn)
	assert.Nil(t, err, "GetDir %v after stale EP err %v", pn, err)
}

func mounted(ts *test.Tstate, pn string) bool {
	return slices.Contains(ts.Mounts(), pn)
}

func unionScans(ts *test.Tstate) int64 {
	st, err := ts.FsLib.FileAPI.Stats()
	assert.Nil(ts.T, err, "Stats err %v", err)
	return st.Path.Counters["NwalkUnion"]
}
