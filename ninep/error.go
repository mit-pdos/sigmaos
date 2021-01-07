package ninep

var (
	ErrBadattach    = &Rerror{"unknown specifier in attach"}
	ErrBadoffset    = &Rerror{"bad offset"}
	ErrBadcount     = &Rerror{"bad count"}
	ErrBotch        = &Rerror{"9P protocol botch"}
	ErrCreatenondir = &Rerror{"create in non-directory"}
	ErrDupfid       = &Rerror{"duplicate fid"}
	ErrDuptag       = &Rerror{"duplicate tag"}
	ErrIsdir        = &Rerror{"is a directory"}
	ErrNocreate     = &Rerror{"create prohibited"}
	ErrNomem        = &Rerror{"out of memory"}
	ErrNoremove     = &Rerror{"remove prohibited"}
	ErrNostat       = &Rerror{"stat prohibited"}
	ErrNotfound     = &Rerror{"file not found"}
	ErrNowrite      = &Rerror{"write prohibited"}
	ErrNowstat      = &Rerror{"wstat prohibited"}
	ErrPerm         = &Rerror{"permission denied"}
	ErrUnknownfid   = &Rerror{"unknown fid"}
	ErrBaddir       = &Rerror{"bad directory in wstat"}
	ErrWalknodir    = &Rerror{"walk in non-directory"}

	// extra errors not part of the normal protocol

	ErrTimeout       = &Rerror{"fcall timeout"}
	ErrUnknownTag    = &Rerror{"unknown tag"}
	ErrUnknownMsg    = &Rerror{"unknown message"}
	ErrUnexpectedMsg = &Rerror{"unexpected message"}
	ErrWalkLimit     = &Rerror{"too many wnames in walk"}
	ErrClosed        = &Rerror{"closed"}
)

// func toError(err error) *Error {
// 	var ecode uint32

// 	ename := err.Error()
// 	if e, ok := err.(syscall.Errno); ok {
// 		ecode = uint32(e)
// 	} else {
// 		ecode = EIO
// 	}

// 	return &Error{ename, ecode}
// }
