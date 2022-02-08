package ninep

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

type Terror uint8

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
	TErrClosed // for pipes
	TErrEOF    // EOF or cannot connect
	TErrBadFcall
	TErrNet
	TErrRetry
	TErrError // to propagate non-sigma errors
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
	case TErrBadFcall:
		return "bad fcall"
	case TErrNet:
		return "network error"
	case TErrRetry:
		return "retry"
	case TErrError:
		return "Error"
	default:
		return "unknown error"
	}
}

type Err struct {
	ErrCode Terror
	Obj     string
}

func MkErr(err Terror, obj interface{}) *Err {
	return &Err{err, fmt.Sprintf("%v", obj)}
}

func (err *Err) Code() Terror {
	return err.ErrCode
}

func (err *Err) Error() string {
	return fmt.Sprintf("%v %v", err.ErrCode, err.Obj)
}

func (err *Err) String() string {
	return err.Error()
}

func (err *Err) Rerror() *Rerror {
	return &Rerror{err.Error()}
}

func IsErrNotfound(error error) bool {
	return strings.HasPrefix(error.Error(), TErrNotfound.String())
}

func ErrNotfoundPath(error error) string {
	return strings.TrimPrefix(error.Error(), TErrNotfound.String()+" ")
}

func IsDirNotFound(error error) bool {
	b := false
	if IsErrNotfound(error) {
		p := Split(strings.TrimPrefix(error.Error(), TErrNotfound.String()))
		b = len(p) > 1
	}
	return b
}

func IsErrExists(error error) bool {
	return strings.HasPrefix(error.Error(), TErrExists.String())
}

func IsErrEOF(error error) bool {
	return strings.HasPrefix(error.Error(), TErrEOF.String())
}

func IsErrStale(error error) bool {
	return strings.HasPrefix(error.Error(), TErrStale.String())
}

func IsErrRetry(error error) bool {
	return strings.HasPrefix(error.Error(), TErrRetry.String())
}

//
// JSON versions
//

func (err *Err) RerrorJson() *Rerror {
	data, error := json.Marshal(*err)
	if error != nil {
		log.Fatalf("FATAL Rerror err %v\n", error)
	}
	return &Rerror{string(data)}
}

func Decode(error error) *Err {
	err := &Err{}
	r := json.Unmarshal([]byte(error.Error()), err)
	if r != nil {
		log.Printf("cannot unmarshal = %v\n", error.Error())
		return nil
	}
	return err
}

func IsErrNotfoundJson(error error) bool {
	err := Decode(error)
	if err == nil {
		return false
	}
	return err.Code() == TErrNotfound
}

func IsDirNotFoundJson(error error) bool {
	b := false
	err := Decode(error)
	if err == nil {
		return b
	}
	if err.Code() == TErrNotfound {
		p := Split(err.Obj)
		b = len(p) > 1
	}
	return b
}
