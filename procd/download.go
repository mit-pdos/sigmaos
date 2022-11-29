package procd

import (
	"path"
	"strings"

	"time"

	db "sigmaos/debug"
	np "sigmaos/sigmap"
	"sigmaos/proc"
	"sigmaos/rand"
)

// Try to download a proc bin from s3.
func (pd *Procd) tryDownloadProcBin(uxBinPath, s3BinPath string) error {
	start := time.Now()
	// Copy the binary from s3 to a temporary file.
	tmppath := path.Join(uxBinPath + "-tmp-" + rand.String(16))
	if err := pd.CopyFile(s3BinPath, tmppath); err != nil {
		return err
	}
	// Rename the temporary file.
	if err := pd.Rename(tmppath, uxBinPath); err != nil {
		// If another procd (or another thread on this procd) already completed the
		// download, then we consider the download successful. Any other error
		// (e.g. ux crashed) is unexpected.
		if !np.IsErrExists(err) {
			return err
		}
		// If someone else completed the download before us, remove the temp file.
		pd.Remove(tmppath)
	}
	db.DPrintf("PROCD", "Took %v to download proc %v", time.Since(start), s3BinPath)
	return nil
}

// Check if we need to download the binary.
func (pd *Procd) needToDownload(uxBinPath, s3BinPath string) bool {
	// If we can't stat the bin through ux, we try to download it.
	_, err := pd.Stat(uxBinPath)
	if err != nil {
		// If we haven't downloaded any procs in this version yet, make a local dir
		// for them.
		versionDir := path.Dir(uxBinPath)
		version := path.Base(versionDir)
		if np.IsErrNotfound(err) && strings.Contains(err.Error(), version) {
			db.DPrintf("PROCD_ERR", "Error first download for version %v: %v", version, err)
			pd.MkDir(versionDir, 0777)
		}
		return true
	}
	return false
}

// XXX Cleanup on procd crashes?
func (pd *Procd) downloadProcBin(program string) {
	pd.Lock()
	defer pd.Unlock()

	uxBinPath := path.Join(np.UXBIN, program)
	s3BinPath := path.Join(np.S3, "~ip", pd.realmbin, program)

	// If we already downloaded the program & it is up-to-date, return.
	if !pd.needToDownload(uxBinPath, s3BinPath) {
		return
	}

	db.DPrintf("PROCD", "Need to download %v", program)

	// Find the number of instances of this proc which have been claimed, and are
	// waiting to be downloaded.
	procCopies := proc.Tcore(0)
	for _, p := range pd.runningProcs {
		if p.attr.Program == program {
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
		if err = pd.tryDownloadProcBin(uxBinPath, s3BinPath); err == nil {
			return
		} else {
			db.DPrintf("PROCD_ERR", "Error tryDownloadProcBin [%v]: %v", s3BinPath, err)
		}
	}
	db.DFatalf("Couldn't download proc bin %v (s3 path: %v) in over %v retries err %v", program, s3BinPath, RETRIES, err)
}
