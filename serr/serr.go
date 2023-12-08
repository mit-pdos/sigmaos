package serr

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"sigmaos/path"
)

type Terror uint32

const (
	TErrBadattach Terror = iota + 1
	TErrBadoffset
	TErrBadcount
	TErrBotch
	TErrCreatenondir
	TErrDupfid
	TErrDuptag
	TErrIsdir
	TErrNocreate
	TErrNomem
	TErrNoremove
	TErrNostat
	TErrNotfound
	TErrNowrite
	TErrNowstat
	TErrPerm
	TErrUnknownfid
	TErrBaddir
	TErrWalknodir

	//
	// sigma protocol errors
	//

	TErrUnreachable
	TErrNotSupported
	TErrInval
	TErrUnknownMsg
	TErrNotDir
	TErrNotFile
	TErrNotSymlink
	TErrNotEmpty
	TErrVersion
	TErrStale
	TErrExists
	TErrClosed // for closed sessions and pipes.
	TErrBadFcall

	//
	// sigma OS errors
	//

	TErrRetry // tell client to retry
	TErrBadFd

	//
	// To propagate non-sigma errors.
	// Must be *last* for String2Err()
	//
	TErrError
)

func (err Terror) String() string {
	switch err {
	case TErrBadattach:
		return "unknown specifier in attach"
	case TErrBadoffset:
		return "bad offset"
	case TErrBadcount:
		return "bad count"
	case TErrBotch:
		return "9P protocol botch"
	case TErrCreatenondir:
		return "create in non-directory"
	case TErrDupfid:
		return "duplicate fid"
	case TErrDuptag:
		return "duplicate tag"
	case TErrIsdir:
		return "is a directory"
	case TErrNocreate:
		return "create prohibited"
	case TErrNomem:
		return "out of memory"
	case TErrNoremove:
		return "remove prohibited"
	case TErrNostat:
		return "stat prohibited"
	case TErrNotfound:
		return "file not found"
	case TErrNowrite:
		return "write prohibited"
	case TErrNowstat:
		return "wstat prohibited"
	case TErrPerm:
		return "permission denied"
	case TErrUnknownfid:
		return "unknown fid"
	case TErrBaddir:
		return "bad directory in wstat"
	case TErrWalknodir:
		return "walk in non-directory"

	// sigma
	case TErrUnreachable:
		return "Unreachable"
	case TErrNotSupported:
		return "not supported"
	case TErrInval:
		return "invalid argument"
	case TErrUnknownMsg:
		return "unknown message"
	case TErrNotDir:
		return "not a directory"
	case TErrNotFile:
		return "not a file"
	case TErrNotSymlink:
		return "not a symlink"
	case TErrNotEmpty:
		return "not empty"
	case TErrVersion:
		return "version mismatch"
	case TErrStale:
		return "stale"
	case TErrExists:
		return "file exists"
	case TErrClosed:
		return "closed"
	case TErrBadFcall:
		return "bad fcall"

	// sigma OS errors
	case TErrRetry:
		return "retry"
	case TErrBadFd:
		return "Bad fd"

	// for passing non-sigma errors through
	case TErrError:
		return "Non-sigma error"

	default:
		return "unknown error"
	}
}

type Err struct {
	ErrCode Terror
	Obj     string
	Err     error
}

func NewErr(err Terror, obj interface{}) *Err {
	return &Err{err, fmt.Sprintf("%v", obj), nil}
}

func NewErrError(error error) *Err {
	return &Err{TErrError, "", error}
}

func (err *Err) Code() Terror {
	return err.ErrCode
}

func (err *Err) Unwrap() error { return err.Err }

func (err *Err) Error() string {
	return fmt.Sprintf("%v %v %v", err.ErrCode, err.Obj, err.Err)
}

func (err *Err) String() string {
	return err.Error()
}

// SigmaOS server couldn't find the requested file
func (err *Err) IsErrNotfound() bool {
	return err.Code() == TErrNotfound
}

// SigmaOS server couldn't find the fid for the requested fid/file
func (err *Err) IsErrUnknownfid() bool {
	return err.Code() == TErrUnknownfid
}

// Maybe the error is because of a symlink or ~
func (err *Err) IsMaybeSpecialElem() bool {
	return err.Code() == TErrNotDir ||
		(err.IsErrNotfound() && path.IsUnionElem(err.Obj))
}

// SigmaOS couldn't reach a server
func (err *Err) IsErrUnreachable() bool {
	return err.Code() == TErrUnreachable
}

// A file is unavailable: either a server on the file's path is
// unreachable or the file is not found
func (err *Err) IsErrUnavailable() bool {
	return err.IsErrUnreachable() || err.IsErrNotfound()
}

func (err *Err) IsErrVersion() bool {
	return err.Code() == TErrVersion
}

func (err *Err) IsErrStale() bool {
	return err.Code() == TErrStale
}

func (err *Err) IsErrSessClosed() bool {
	return err.Code() == TErrClosed && strings.Contains(err.Error(), "sess")
}

func (err *Err) IsErrRetry() bool {
	return err.Code() == TErrRetry
}

func (err *Err) IsErrExists() bool {
	return err.Code() == TErrExists
}

func (err *Err) ErrPath() string {
	if err.IsErrNotfound() {
		return err.Obj
	} else if err.IsErrUnreachable() {
		return err.Obj
	} else {
		return ""
	}
}

func IsErrorUnavailable(error error) bool {
	var err *Err
	if errors.As(error, &err) {
		return err.IsErrUnavailable()
	}
	return false
}

func IsErrCode(error error, code Terror) bool {
	var err *Err
	if errors.As(error, &err) {
		return err.Code() == code
	}
	return false
}

func errnoToErr(errno syscall.Errno, err error, name string) *Err {
	switch errno {
	case syscall.ENOENT:
		return NewErr(TErrNotfound, name)
	case syscall.EEXIST:
		return NewErr(TErrExists, name)
	default:
		return NewErrError(err)
	}
}

func UxErrnoToErr(err error, name string) *Err {
	switch e := err.(type) {
	case *os.LinkError:
		return errnoToErr(e.Err.(syscall.Errno), err, name)
	case *os.PathError:
		return errnoToErr(e.Err.(syscall.Errno), err, name)
	case syscall.Errno:
		return errnoToErr(e, err, name)
	default:
		return NewErrError(err)
	}
}
