package retry

import (
	"fmt"
	"time"

	db "sigmaos/debug"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

func retryLoop(i int, f func(i int, pn sp.Tsigmapath) error, src sp.Tsigmapath) error {
	var r error
	for i := 0; i < sp.Conf.FsLib.MAX_RETRY; i++ {
		// Return if successful. Else, retry
		if err := f(i, src); err == nil {
			return nil
		} else {
			db.DPrintf(db.ALWAYS, "download %q err %v", src, err)
			r = err
			if serr.IsErrCode(err, serr.TErrNotfound) {
				break
			}
		}
	}
	return fmt.Errorf("retryLoop: couldn't do %T for %q in %d retries err %v", f, src, sp.Conf.FsLib.MAX_RETRY, r)
}

func RetryPaths(paths []sp.Tsigmapath, f func(i int, pn sp.Tsigmapath) error) error {
	var r error
	for i, pp := range paths {
		if err := retryLoop(i, f, pp); err == nil {
			return nil
		} else {
			db.DPrintf(db.ALWAYS, "download pp %q err %v", pp, err)
			r = err
		}
	}
	return r
}

// Repeat f a number of times if okf says ok. Between each iteration
// sleep for duration d.
func Repeat(f func() error, okf func(error) bool, d time.Duration) error {
	for i := 0; true; i++ {
		if err := f(); err == nil {
			return nil
		} else if !okf(err) {
			return err
		} else if i >= sp.Conf.Path.MAX_RESOLVE_RETRY {
			return err
		}
		time.Sleep(d)
	}
	return nil
}

func RepeatDefDur(f func() error, okf func(error) bool) error {
	return Repeat(f, okf, sp.Conf.Path.RESOLVE_TIMEOUT)
}

// RetryAtMostOnce is intended for functions f that want to retry in case named
// is down until it is responding again, but not execute an op twice;
// that is fail on ErrIO.
func RetryAtMostOnce(f func() error) error {
	return Repeat(f, serr.IsErrorWalkOK, sp.Conf.Path.RESOLVE_TIMEOUT)
}

// RetryAtLeastOnce is intended for functions f that want to retry in
// case named is down until it is responding again; it may execute an
// operation twice, because it retries on ErrIO.
func RetryAtLeastOnce(f func() error) error {
	return Repeat(f, serr.IsErrorRetryOpenOK, sp.Conf.Path.RESOLVE_TIMEOUT)
}
