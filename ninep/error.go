package ninep

import (
	"fmt"
	"strings"
)

type Terror int8

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

	// sigma
	TErrNotSupported
	TErrInval
	TErrUnknownMsg
	TErrNotDir
	TErrNotFile
	TErrNotEmpty
	TErrVersion
	TErrStale
	TErrUnknownFence
	TErrInvalidSession
	TErrExists
	TErrClosed
	TErrEOF
	TErrError // propagate error
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
	case TErrNotEmpty:
		return "not empty"
	case TErrVersion:
		return "version mismatch"
	case TErrStale:
		return "stale fence"
	case TErrUnknownFence:
		return "unknown fence"
	case TErrInvalidSession:
		return "invalid session"
	case TErrExists:
		return "exists"
	case TErrEOF:
		return "EOF"
	case TErrError:
		return "Error"
	default:
		return "unknown error"
	}
}

type Err struct {
	err Terror
	obj string
}

func MkErr(err Terror, obj interface{}) *Err {
	return &Err{err, fmt.Sprintf("%v", obj)}
}

func (err *Err) Err() Terror {
	return err.err
}

func (err *Err) Error() string {
	return fmt.Sprintf("%v %v", err.err, err.obj)
}

func (err *Err) Rerror() *Rerror {
	return &Rerror{err.Error()}
}

func IsDirNotFound(err string) bool {
	b := false
	if strings.HasPrefix(err, "file not found") {
		p := Split(strings.TrimPrefix(err, "file not found "))
		b = len(p) > 1
	}
	return b
}
