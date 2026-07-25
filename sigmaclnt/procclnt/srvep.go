package procclnt

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

// Endpoint caching for the servers under a union directory (e.g., name/ux,
// name/s3). A parent discovers the endpoints once and stamps them onto the
// procs it spawns; a child that wants them mounts them explicitly with
// MountCachedEndpoint/MountCachedLocalSrv. That saves the child the walk
// through named it would otherwise do to find the server: read the union
// directory, read the per-kernel endpoint file, then attach.
//
// Both halves are opt-in. Nothing here fires unless a parent asked for it
// *and* the child asked for it, and neither half is a correctness
// requirement: a child whose parent cached nothing, or whose cached endpoint
// is stale, walks the namespace as before.

// SrvEPCache caches the endpoints of the servers under the union directory
// unionpn. Safe for concurrent use.
type SrvEPCache struct {
	// discoverMu serializes discovery, so that concurrent callers issue one
	// round of RPCs rather than one each. Never held while mu is held.
	discoverMu sync.Mutex

	mu         sync.Mutex
	fsl        *fslib.FsLib
	unionpn    string
	eps        map[string]*sp.Tendpoint // "name/ux/<kid>" -> EP
	discovered bool
	refreshing bool
}

func NewSrvEPCache(fsl *fslib.FsLib, unionpn string) *SrvEPCache {
	return &SrvEPCache{fsl: fsl, unionpn: unionpn}
}

func (c *SrvEPCache) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return fmt.Sprintf("{unionpn %v neps %d}", c.unionpn, len(c.eps))
}

// Endpoints returns the endpoints of the servers under the union directory,
// discovering them on the first call. The returned map must not be modified:
// it is shared with other callers, and Refresh replaces it wholesale rather
// than mutating it.
func (c *SrvEPCache) Endpoints() (map[string]*sp.Tendpoint, error) {
	if eps, ok := c.cached(); ok {
		return eps, nil
	}

	c.discoverMu.Lock()
	defer c.discoverMu.Unlock()

	// Another thread may have discovered while we waited for discoverMu.
	if eps, ok := c.cached(); ok {
		return eps, nil
	}
	eps, err := c.discover()
	if err != nil {
		return nil, err
	}
	c.publish(eps)
	return eps, nil
}

// CacheEndpoints stamps the endpoints onto p's ProcEnv, so that p can mount
// them without looking them up. Call before Spawn. Issues no RPCs once the
// endpoints have been discovered, so it is safe on the spawn path; warm the
// cache with Endpoints() first to keep it that way.
func (c *SrvEPCache) CacheEndpoints(p *proc.Proc) error {
	eps, err := c.Endpoints()
	if err != nil {
		return err
	}
	for pn, ep := range eps {
		p.SetCachedEndpoint(pn, ep)
	}
	db.DPrintf(db.PROCCLNT, "CacheEndpoints %v: %d eps -> %v", c.unionpn, len(eps), p.GetPid())
	return nil
}

// Refresh re-discovers the endpoints in the background and swaps them in when
// done, so that a refresh never blocks a concurrent CacheEndpoints on named:
// callers keep seeing the current endpoints until the swap. A refresh that is
// already in flight isn't duplicated.
func (c *SrvEPCache) Refresh() {
	c.mu.Lock()
	if c.refreshing {
		c.mu.Unlock()
		return
	}
	c.refreshing = true
	c.mu.Unlock()

	go func() {
		c.discoverMu.Lock()
		eps, err := c.discover()
		c.discoverMu.Unlock()

		c.mu.Lock()
		defer c.mu.Unlock()
		c.refreshing = false
		if err != nil {
			// Keep the endpoints we have; they may still be good, and a
			// child that finds one stale falls back to walking.
			db.DPrintf(db.PROCCLNT_ERR, "SrvEPCache.Refresh %v err %v", c.unionpn, err)
			return
		}
		c.eps = eps
		c.discovered = true
	}()
}

func (c *SrvEPCache) cached() (map[string]*sp.Tendpoint, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.eps, c.discovered
}

func (c *SrvEPCache) publish(eps map[string]*sp.Tendpoint) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eps = eps
	c.discovered = true
}

// discover reads the union directory and every server's endpoint file,
// concurrently. Servers whose endpoint can't be read are skipped rather than
// failing the whole discovery: a partial set still saves the children it
// covers, and the rest walk the namespace as before.
func (c *SrvEPCache) discover() (map[string]*sp.Tendpoint, error) {
	start := time.Now()
	sts, err := c.fsl.GetDir(c.unionpn)
	if err != nil {
		db.DPrintf(db.PROCCLNT_ERR, "SrvEPCache.discover GetDir %v err %v", c.unionpn, err)
		return nil, err
	}
	type result struct {
		pn string
		ep *sp.Tendpoint
	}
	ch := make(chan *result, len(sts))
	names := sp.Names(sts)
	for _, n := range names {
		go func(n string) {
			pn := filepath.Join(c.unionpn, n)
			ep, err := c.fsl.ReadEndpoint(pn)
			if err != nil {
				db.DPrintf(db.PROCCLNT_ERR, "SrvEPCache.discover ReadEndpoint %v err %v", pn, err)
				ch <- nil
				return
			}
			ch <- &result{pn, ep}
		}(n)
	}
	eps := make(map[string]*sp.Tendpoint, len(names))
	for range names {
		if r := <-ch; r != nil {
			eps[r.pn] = r.ep
		}
	}
	db.DPrintf(db.PROCCLNT, "SrvEPCache.discover %v: %d/%d eps lat %v", c.unionpn, len(eps), len(names), time.Since(start))
	return eps, nil
}

// MountCachedEndpoint mounts the server whose endpoint the parent cached for
// pn at pn, so that walks under pn are served by the mount table instead of
// going through named. Reports whether an endpoint was cached for pn;
// callers that get false (or an error) can proceed unchanged, they just pay
// the walk. Idempotent.
func MountCachedEndpoint(fsl *fslib.FsLib, pn string) (bool, error) {
	ep, ok := fsl.ProcEnv().GetCachedEndpoint(pn)
	if !ok {
		db.DPrintf(db.PROCCLNT, "MountCachedEndpoint %v: no cached EP", pn)
		return false, nil
	}
	if err := mountSrvRoot(fsl, ep, pn); err != nil {
		return true, err
	}
	return true, nil
}

// MountCachedLocalSrv mounts this kernel's instance of the union-dir'd
// service unionpn (e.g., sp.UX) from the endpoint the parent cached for it,
// at both <unionpn>/<kernelID> and <unionpn>/~local, so that either spelling
// is served by the mount table. Reports whether an endpoint was cached.
func MountCachedLocalSrv(fsl *fslib.FsLib, unionpn string) (bool, error) {
	kid := fsl.ProcEnv().GetKernelID()
	if kid == sp.NOT_SET || kid == "" {
		return false, nil
	}
	pn := filepath.Join(unionpn, kid)
	ep, ok := fsl.ProcEnv().GetCachedEndpoint(pn)
	if !ok {
		db.DPrintf(db.PROCCLNT, "MountCachedLocalSrv %v: no cached EP", pn)
		return false, nil
	}
	if err := mountSrvRoot(fsl, ep, pn); err != nil {
		return true, err
	}
	// Mount the same server under ~local too: ~local names the instance on
	// this kernel, which is exactly what we just mounted, and the mount table
	// matches path components literally (so a walk of <unionpn>/~local/...
	// resolves here without the union scan that would otherwise read the
	// endpoint file from named). Reuses the connection the mount above
	// established.
	if err := mountSrvRoot(fsl, ep, filepath.Join(unionpn, sp.LOCAL)); err != nil {
		return true, err
	}
	return true, nil
}

// mountSrvRoot mounts the root of the server named by ep at pn.
func mountSrvRoot(fsl *fslib.FsLib, ep *sp.Tendpoint, pn string) error {
	start := time.Now()
	if err := fsl.MountTree(ep, "", pn); err != nil {
		// Most likely a stale endpoint (the server restarted). MountTree
		// removes the mount point it failed to attach, so a later walk of pn
		// finds the server through named as usual.
		db.DPrintf(db.PROCCLNT_ERR, "mountSrvRoot [%v] %v err %v", ep, pn, err)
		return err
	}
	db.DPrintf(db.PROCCLNT, "mountSrvRoot [%v] %v lat %v", ep, pn, time.Since(start))
	return nil
}
