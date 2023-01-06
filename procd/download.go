package procd

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

// Procd caches binary locally at the sigma patname cacheDir().  When
// running ./install.sh locally, it will copy binaries in the cache
// and no downloads are necessary.  XXX make cache searchpath aware
func cacheDir(pn string) string {
	return path.Join(sp.UXBIN, "user", path.Base(pn))
}

// Try to download a proc at pn to local Ux dir.
func (pd *Procd) tryDownloadProcBin(pn string) error {
	start := time.Now()
	db.DPrintf(db.PROCD, "tryDownloadProcBin %s\n", pn)
	cachePn := cacheDir(pn)
	// Copy the binary from s3 to a temporary file.
	tmppath := path.Join(cachePn + "-tmp-" + rand.String(16))
	if err := pd.CopyFile(pn, tmppath); err != nil {
		return err
	}
	// Rename the temporary file.
	if err := pd.Rename(tmppath, cachePn); err != nil {
		// If another procd (or another thread on this procd) already completed the
		// download, then we consider the download successful. Any other error
		// (e.g. ux crashed) is unexpected.
		if !serr.IsErrExists(err) {
			return err
		}
		// If someone else completed the download before us, remove the temp file.
		pd.Remove(tmppath)
	}
	db.DPrintf(db.PROCD, "Took %v to download proc %v", time.Since(start), pn)
	return nil
}

// Check if we need to download the binary.  XXX check that
// pn isn't newer than cached version.
func (pd *Procd) needToDownload(pn string) bool {
	// If we can't stat the bin through ux, we try to download it.
	cachePn := cacheDir(pn)
	_, err := pd.Stat(cachePn)
	db.DPrintf(db.PROCD, "uxp %s err %v\n", cachePn, err)
	if err != nil {
		return true
	}
	return false
}

// XXX Cleanup on procd crashes?
func (pd *Procd) downloadProcBin(p *proc.Proc) error {
	pd.Lock()
	defer pd.Unlock()

	searchpath, ok := p.LookupEnv(proc.SIGMAPATH)
	if !ok {
		return fmt.Errorf("downloadProcBin: no search path")
	}
	paths := filepath.SplitList(searchpath)
	var r error
	for _, pp := range paths {
		if err := pd.downloadProcPath(path.Join(pp, p.Program)); err == nil {
			return nil
		} else {
			r = err
		}
	}
	return r
}

func (pd *Procd) downloadProcPath(pn string) error {
	if !pd.needToDownload(pn) {
		db.DPrintf(db.PROCD, "Program cached at %v", cacheDir(pn))
		return nil
	}

	// Find the number of instances of this proc which have been claimed, and are
	// waiting to be downloaded.
	procCopies := proc.Tcore(0)
	for _, pp := range pd.runningProcs {
		if pp.attr.Program == path.Base(pn) {
			procCopies++
		}
	}
	// Note that a proc is downloading, so we don't pull procs too aggressively.
	// It's utilization won't have been measured yet.
	pd.procsDownloading += procCopies
	defer func() {
		pd.procsDownloading -= procCopies
	}()

	// May need to retry if ux crashes.
	var err error
	RETRIES := 100
	for i := 0; i < RETRIES && !pd.done; i++ {
		// Return if successful. Else, retry
		if err = pd.tryDownloadProcBin(pn); err == nil {
			return nil
		} else {
			db.DPrintf(db.PROCD_ERR, "Error tryDownloadProcBin [%v]: %v", pn, err)
			if serr.IsErrNotfound(err) {
				break
			}
		}
	}
	return fmt.Errorf("downloadProbBin: Couldn't download %v (s3 path: %v) err %v", pn, RETRIES, err)
}
