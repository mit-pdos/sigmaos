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

	// ulambda
	ErrNotSupported = &Rerror{"not supported"}
	ErrInval        = &Rerror{"invalid argument"}
	ErrUnknownMsg   = &Rerror{"unknown message"}
	ErrNotDir       = &Rerror{"not a directory"}
	ErrNotFile      = &Rerror{"not a file"}
	ErrClunked      = &Rerror{"clunked by server"}
)
