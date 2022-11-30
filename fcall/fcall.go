package fcall

import (
	"strconv"
)

type Tfcall uint8
type Tsession uint64
type Tclient uint64

// NoSession signifies the fcall came from a wire-compatible peer
const NoSession Tsession = ^Tsession(0)

func (s Tsession) String() string {
	return strconv.FormatUint(uint64(s), 16)
}

type Fcall interface {
	Type() Tfcall
	Session() Tsession
	Client() Tclient
}

const (

	//
	// 9P
	//

	TTversion Tfcall = iota + 100
	TRversion
	TTauth
	TRauth
	TTattach
	TRattach
	TTerror
	TRerror
	TTflush
	TRflush
	TTwalk
	TRwalk
	TTopen
	TRopen
	TTcreate
	TRcreate
	TTread
	TRread
	TTwrite
	TRwrite
	TTclunk
	TRclunk
	TTremove
	TRremove
	TTstat
	TRstat
	TTwstat
	TRwstat

	//
	// SigmaP
	//

	TTreadV
	TTwriteV
	TTwatch
	TTrenameat
	TRrenameat
	TTremovefile
	TTgetfile
	TRgetfile
	TTsetfile
	TTputfile
	TTdetach
	TRdetach
	TTheartbeat
	TRheartbeat
	TTwriteread
	TRwriteread
)

func (fct Tfcall) String() string {
	switch fct {
	case TTversion:
		return "Tversion"
	case TRversion:
		return "Rversion"
	case TTauth:
		return "Tauth"
	case TRauth:
		return "Rauth"
	case TTattach:
		return "Tattach"
	case TRattach:
		return "Rattach"
	case TTerror:
		return "Terror"
	case TRerror:
		return "Rerror"
	case TTflush:
		return "Tflush"
	case TRflush:
		return "Rflush"
	case TTwalk:
		return "Twalk"
	case TRwalk:
		return "Rwalk"
	case TTopen:
		return "Topen"
	case TRopen:
		return "Ropen"
	case TTcreate:
		return "Tcreate"
	case TRcreate:
		return "Rcreate"
	case TTread:
		return "Tread"
	case TRread:
		return "Rread"
	case TTwrite:
		return "Twrite"
	case TRwrite:
		return "Rwrite"
	case TTclunk:
		return "Tclunk"
	case TRclunk:
		return "Rclunk"
	case TTremove:
		return "Tremove"
	case TRremove:
		return "Rremove"
	case TTstat:
		return "Tstat"
	case TRstat:
		return "Rstat"
	case TTwstat:
		return "Twstat"
	case TRwstat:
		return "Rwstat"

	case TTreadV:
		return "TreadV"
	case TTwriteV:
		return "TwriteV"
	case TTwatch:
		return "Twatch"
	case TTrenameat:
		return "Trenameat"
	case TRrenameat:
		return "Rrenameat"
	case TTremovefile:
		return "Tremovefile"
	case TTgetfile:
		return "Tgetfile"
	case TRgetfile:
		return "Rgetfile"
	case TTsetfile:
		return "Tsetfile"
	case TTputfile:
		return "Tputfile"
	case TTdetach:
		return "Tdetach"
	case TRdetach:
		return "Rdetach"
	case TTheartbeat:
		return "Theartbeat"
	case TRheartbeat:
		return "Rheartbeat"
	case TTwriteread:
		return "Twriteread"
	case TRwriteread:
		return "Rwriteread"
	default:
		return "Tunknown"
	}
}
