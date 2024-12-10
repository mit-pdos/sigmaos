package fslib

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

func retryLoop(i int, f func(i int, pn string) error, src string) error {
	var r error
	for i := 0; i < sp.Conf.FsLib.MAX_RETRY; i++ {
		// Return if successful. Else, retry
		if err := f(i, src); err == nil {
			return nil
		} else {
			db.DPrintf(db.ERROR, "download %q err %v", src, err)
			r = err
			if serr.IsErrCode(err, serr.TErrNotfound) {
				break
			}
		}
	}
	return fmt.Errorf("retryLoop: couldn't do %T for %q in %d retries err %v", f, src, sp.Conf.FsLib.MAX_RETRY, r)
}

func RetryPaths(paths []string, f func(i int, pn string) error) error {
	var r error
	for i, pp := range paths {
		if err := retryLoop(i, f, pp); err == nil {
			return nil
		} else {
			db.DPrintf(db.ERROR, "download pp %q err %v", pp, err)
			r = err
		}
	}
	return r
}