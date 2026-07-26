package procclnt

import (
	"fmt"
	"path/filepath"
	"slices"
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
	gen        int // bumped on every publish, to tell log lines apart
}

func NewSrvEPCache(fsl *fslib.FsLib, unionpn string) *SrvEPCache {
	return &SrvEPCache{fsl: fsl, unionpn: unionpn}
}

func (c *SrvEPCache) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return fmt.Sprintf("{unionpn %v neps %d gen %d refreshing %t}", c.unionpn, len(c.eps), c.gen, c.refreshing)
}

// Srvs returns the pathnames of the servers whose endpoints are cached, in
// sorted order, for logging.
func (c *SrvEPCache) Srvs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return srvPaths(c.eps)
}

func srvPaths(eps map[string]*sp.Tendpoint) []string {
	pns := make([]string, 0, len(eps))
	for pn := range eps {
		pns = append(pns, pn)
	}
	slices.Sort(pns)
	return pns
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
	db.DPrintf(db.PROCCLNT_EPCACHE, "CacheEndpoints %v: %d eps -> %v", c.unionpn, len(eps), p.GetPid())
	return nil
}

// Refresh re-discovers the endpoints in the background and swaps them in when
// done, so that a refresh never blocks a concurrent CacheEndpoints on named:
// callers keep seeing the current endpoints until the swap. A refresh that is
// already in flight isn't duplicated.
func (c *SrvEPCache) Refresh() {
	c.mu.Lock()
	if c.refreshing {
		db.DPrintf(db.PROCCLNT_EPCACHE, "SrvEPCache.Refresh %v: already in flight, skip", c.unionpn)
		c.mu.Unlock()
		return
	}
	c.refreshing = true
	c.mu.Unlock()

	db.DPrintf(db.PROCCLNT_EPCACHE, "SrvEPCache.Refresh start %v", c)
	go func() {
		start := time.Now()
		c.discoverMu.Lock()
		eps, err := c.discover()
		c.discoverMu.Unlock()

		c.mu.Lock()
		defer c.mu.Unlock()
		c.refreshing = false
		if err != nil {
			// Keep the endpoints we have; they may still be good, and a
			// child that finds one stale falls back to walking.
			db.DPrintf(db.PROCCLNT_EPCACHE_ERR, "SrvEPCache.Refresh %v failed after %v, keeping %d eps: %v", c.unionpn, time.Since(start), len(c.eps), err)
			return
		}
		added, removed, changed := diffEPs(c.eps, eps)
		c.setLocked(eps)
		if len(added)+len(removed)+len(changed) == 0 {
			db.DPrintf(db.PROCCLNT_EPCACHE, "SrvEPCache.Refresh %v done in %v: unchanged, %d eps (gen %d)", c.unionpn, time.Since(start), len(eps), c.gen)
			return
		}
		// The set of servers, or one of their endpoints, changed underneath a
		// running job, which means procs already spawned may be holding a
		// stale endpoint.
		db.DPrintf(db.PROCCLNT_EPCACHE, "SrvEPCache.Refresh %v done in %v (gen %d): %d eps, added %v removed %v changed %v", c.unionpn, time.Since(start), c.gen, len(eps), added, removed, changed)
	}()
}

// diffEPs reports which server pathnames were added, removed, and had their
// endpoint change between two discoveries.
func diffEPs(old, cur map[string]*sp.Tendpoint) (added, removed, changed []string) {
	for _, pn := range srvPaths(cur) {
		oldEP, ok := old[pn]
		if !ok {
			added = append(added, pn)
		} else if oldEP.String() != cur[pn].String() {
			changed = append(changed, pn)
		}
	}
	for _, pn := range srvPaths(old) {
		if _, ok := cur[pn]; !ok {
			removed = append(removed, pn)
		}
	}
	return added, removed, changed
}

func (c *SrvEPCache) cached() (map[string]*sp.Tendpoint, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.eps, c.discovered
}

func (c *SrvEPCache) publish(eps map[string]*sp.Tendpoint) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setLocked(eps)
}

// setLocked installs a freshly discovered set of endpoints. Caller holds mu.
func (c *SrvEPCache) setLocked(eps map[string]*sp.Tendpoint) {
	c.eps = eps
	c.discovered = true
	c.gen++
}

// discover reads the union directory and every server's endpoint file,
// concurrently. Servers whose endpoint can't be read are skipped rather than
// failing the whole discovery: a partial set still saves the children it
// covers, and the rest walk the namespace as before.
func (c *SrvEPCache) discover() (map[string]*sp.Tendpoint, error) {
	start := time.Now()
	sts, err := c.fsl.GetDir(c.unionpn)
	if err != nil {
		db.DPrintf(db.PROCCLNT_EPCACHE_ERR, "SrvEPCache.discover GetDir %v err %v", c.unionpn, err)
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
				db.DPrintf(db.PROCCLNT_EPCACHE_ERR, "SrvEPCache.discover ReadEndpoint %v err %v", pn, err)
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
	db.DPrintf(db.PROCCLNT_EPCACHE, "SrvEPCache.discover %v: %d/%d eps lat %v srvs %v", c.unionpn, len(eps), len(names), time.Since(start), srvPaths(eps))
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
		db.DPrintf(db.PROCCLNT_EPCACHE, "MountCachedEndpoint %v: no cached EP", pn)
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
		db.DPrintf(db.PROCCLNT_EPCACHE, "MountCachedLocalSrv %v: no cached EP", pn)
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
		db.DPrintf(db.PROCCLNT_EPCACHE_ERR, "mountSrvRoot [%v] %v err %v", ep, pn, err)
		return err
	}
	db.DPrintf(db.PROCCLNT_EPCACHE, "mountSrvRoot [%v] %v lat %v", ep, pn, time.Since(start))
	return nil
}
