package procmgr

import (
	"fmt"
	"path"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

const (
	N_DOWNLOAD_RETRIES = 100
)

func (mgr *ProcMgr) uxBinPath() string {
	return path.Join(sp.UX, mgr.kernelId, "bin")
}

// ProcMgr caches binary locally. There is a cache directory for each realm.
func (mgr *ProcMgr) cachePath(realm sp.Trealm, prog string) string {
	return path.Join(mgr.uxBinPath(), "user", "realms", realm.String(), prog)
}

func (mgr *ProcMgr) setupUserBinCacheL(realm sp.Trealm) error {
	if _, ok := mgr.cachedProcBins[realm]; !ok {
		db.DPrintf(db.PROCMGR, "Make user bin cache for realm %v", realm)
		cachePn := path.Dir(mgr.cachePath(realm, "PROGRAM"))
		// Make a dir to cache the realm's binaries.
		if err := mgr.rootsc.MkDir(cachePn, 0777); err != nil {
			db.DPrintf(db.ERROR, "Error MkDir cache dir [%v]: %v", cachePn, err)
			return err
		}
		mgr.cachedProcBins[realm] = make(map[string]bool)
	}
	return nil
}

// Returns true if the proc is already cached.
// XXX check timestamps/versions?
func (mgr *ProcMgr) alreadyCached(realm sp.Trealm, prog string) bool {
	return mgr.cachedProcBins[realm][prog]
}

func (mgr *ProcMgr) downloadProc(p *proc.Proc) error {
	// Privileged procs' bins should be part of the base image.
	if p.IsPrivileged() {
		return nil
	}
	// Download the bin from s3, if it isn't already cached locally.
	if err := mgr.downloadProcBin(p.GetRealm(), p.GetProgram(), p.GetBuildTag()); err != nil {
		db.DPrintf(db.ERROR, "failed to download proc err:%v proc:%v", err, p)
		return fmt.Errorf("Unable to download proc: %v", err)
	}
	return nil
}

// Lock to ensure the bin is downloaded only once, even if multiple copies of
// the proc are starting up on the same schedd.
func (mgr *ProcMgr) downloadProcBin(realm sp.Trealm, prog, buildTag string) error {
	mgr.Lock()
	defer mgr.Unlock()

	// If already cached, return immediately.
	if mgr.alreadyCached(realm, prog) {
		return nil
	}
	commonBins := path.Join(mgr.uxBinPath(), "user", "common")
	// Search order:
	// 1. Try to copy from the local bin cache (user bins will be here when built locally).
	// 2. Try the global version repo.
	paths := []string{
		commonBins,
		path.Join(sp.S3, "~local", buildTag, "/bin"),
	}
	// For user bins, go straight to S3 instead of checking locally first.
	if sp.Target != "local" && prog != "named" && prog != "spawn-latency-ux" {
		paths = paths[1:]
	}
	var err error
	for _, pp := range paths {
		db.DPrintf(db.PROCMGR, "Download buildtag %v pp %v prog %v", buildTag, pp, prog)
		if e := mgr.downloadProcPath(realm, pp, prog); e == nil {
			mgr.cachedProcBins[realm][prog] = true
			return nil
		} else {
			err = e
		}
	}
	return err
}

func (mgr *ProcMgr) downloadProcPath(realm sp.Trealm, from, prog string) error {
	// May need to retry if ux crashes.
	var i int
	var err error
	for i = 0; i < N_DOWNLOAD_RETRIES; i++ {
		// Return if successful. Else, retry
		if err = mgr.tryDownloadProcPath(realm, from, prog); err == nil {
			return nil
		} else {
			db.DPrintf(db.PROCMGR_ERR, "Error tryDownloadProcBin [%v]: %v", path.Join(from, prog), err)
			if serr.IsErrCode(err, serr.TErrNotfound) {
				break
			}
		}
	}
	return fmt.Errorf("downloadProcPath: Couldn't download %v in %v retries err %v", path.Join(from, prog), i, err)
}

// Try to download a proc at pn to local Ux dir. May fail if ux crashes.
func (mgr *ProcMgr) tryDownloadProcPath(realm sp.Trealm, from, prog string) error {
	src := path.Join(from, prog)
	start := time.Now()
	db.DPrintf(db.PROCMGR, "tryDownloadProcPath %s", src)
	cachePn := mgr.cachePath(realm, prog)
	// Copy the binary from s3 to a temporary file.
	tmppath := path.Join(cachePn + "-tmp-" + rand.String(8))
	if err := mgr.rootsc.CopyFile(src, tmppath); err != nil {
		return err
	}
	// Rename the temporary file.
	if err := mgr.rootsc.Rename(tmppath, cachePn); err != nil {
		// If another schedd (or another thread on this schedd) already completed the
		// download, then we consider the download successful. Any other error
		// (e.g. ux crashed) is unexpected.
		if err != nil && !serr.IsErrCode(err, serr.TErrExists) {
			return err
		}
		// If someone else completed the download before us, remove the temp file.
		mgr.rootsc.Remove(tmppath)
	}
	db.DPrintf(db.PROCMGR, "Took %v to download proc %v", time.Since(start), src)
	db.DPrintf(db.SPAWN_LAT, "Took %v to download proc %v", time.Since(start), src)
	return nil
}
