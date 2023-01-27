package procmgr

import (
	"fmt"
	"path"
	"path/filepath"

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

// ProcMgr caches binary locally at the sigma pathname cacheDir().  When
// running ./build.sh it will copy binaries in the cache and no
// downloads are necessary.  XXX make cache searchpath aware
func cacheDir(pn string) string {
	return path.Join(sp.UXBIN, "user", path.Base(pn))
}

func (mgr *ProcMgr) downloadProc(p *proc.Proc) {
	// Privileged procs' bins should be part of the base image.
	if p.IsPrivilegedProc() {
		return
	}
	// Download the bin from s3, if it isn't already cached locally.
	if err := mgr.downloadProcBin(p); err != nil {
		db.DFatalf("failed to download proc %v", p)
	}
}

// Lock to ensure the bin is downloaded only once, even if multiple copies of
// the proc are starting up on the same schedd.
func (mgr *ProcMgr) downloadProcBin(p *proc.Proc) error {
	mgr.Lock()
	defer mgr.Unlock()

	searchpath, ok := p.LookupEnv(proc.SIGMAPATH)
	if !ok {
		return fmt.Errorf("downloadProcBin: no search path")
	}
	paths := filepath.SplitList(searchpath)
	var err error
	for _, pp := range paths {
		if e := mgr.downloadProcPath(path.Join(pp, p.Program)); e == nil {
			return nil
		} else {
			err = e
		}
	}
	return err
}

// Returns true if the proc is already cached.
// XXX check timestamps/versions?
func (mgr *ProcMgr) alreadyCached(pn string) bool {
	// If we can't stat the bin through ux, we try to download it.
	cachePn := cacheDir(pn)
	_, err := mgr.rootsc.Stat(cachePn)
	db.DPrintf(db.PROCMGR, "uxp %s err %v\n", cachePn, err)
	if err != nil {
		return true
	}
	return false
}

func (mgr *ProcMgr) downloadProcPath(pn string) error {
	if !mgr.alreadyCached(pn) {
		db.DPrintf(db.PROCMGR, "Program cached at %v", cacheDir(pn))
		return nil
	}

	// May need to retry if ux crashes.
	var err error
	for i := 0; i < N_DOWNLOAD_RETRIES; i++ {
		// Return if successful. Else, retry
		if err = mgr.tryDownloadProcPath(pn); err == nil {
			return nil
		} else {
			db.DPrintf(db.PROCMGR_ERR, "Error tryDownloadProcBin [%v]: %v", pn, err)
			if serr.IsErrNotfound(err) {
				break
			}
		}
	}
	return fmt.Errorf("downloadProcPath: Couldn't download %v in %v retries err %v", pn, N_DOWNLOAD_RETRIES, err)
}

// Try to download a proc at pn to local Ux dir. May fail if ux crashes.
func (mgr *ProcMgr) tryDownloadProcPath(pn string) error {
	start := time.Now()
	db.DPrintf(db.PROCMGR, "tryDownloadProcPath %s", pn)
	cachePn := cacheDir(pn)
	// Copy the binary from s3 to a temporary file.
	tmppath := path.Join(cachePn + "-tmp-" + rand.String(16))
	if err := mgr.rootsc.CopyFile(pn, tmppath); err != nil {
		return err
	}
	// Rename the temporary file.
	if err := mgr.rootsc.Rename(tmppath, cachePn); err != nil {
		// If another schedd (or another thread on this schedd) already completed the
		// download, then we consider the download successful. Any other error
		// (e.g. ux crashed) is unexpected.
		if !serr.IsErrExists(err) {
			return err
		}
		// If someone else completed the download before us, remove the temp file.
		mgr.rootsc.Remove(tmppath)
	}
	db.DPrintf(db.PROCMGR, "Took %v to download proc %v", time.Since(start), pn)
	return nil
}
