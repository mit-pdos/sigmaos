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

type Tmsg interface {
	Type() Tfcall
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
	TRerror9P
	TTflush
	TRflush
	TTwalk
	TRwalk
	TTopen9P
	TRopen
	TTcreate9P
	TRcreate
	TTread
	TRread9P
	TTwrite
	TRwrite
	TTclunk
	TRclunk
	TTremove
	TRremove
	TTstat
	TRstat9P
	TTwstat9P
	TRwstat

	//
	// SigmaP
	//
	TRerror
	TTopen
	TTcreate
	TTreadV
	TRread
	TTwriteV
	TTwatch
	TRstat
	TTwstat
	TTrenameat
	TRrenameat
	TTremovefile
	TTgetfile
	TTsetfile
	TTputfile
	TTdetach
	TRdetach
	TTheartbeat
	TRheartbeat
	TTwriteread
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
	case TRerror9P:
		return "Rerror9P"
	case TTflush:
		return "Tflush"
	case TRflush:
		return "Rflush"
	case TTwalk:
		return "Twalk"
	case TRwalk:
		return "Rwalk"
	case TTopen9P:
		return "Topen9P"
	case TRopen:
		return "Ropen"
	case TTcreate9P:
		return "Tcreate"
	case TRcreate:
		return "Rcreate"
	case TTread:
		return "Tread9P"
	case TRread:
		return "Rread9P"
	case TTwrite:
		return "Twrite9P"
	case TRwrite:
		return "Rwrite9P"
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
	case TRstat9P:
		return "Rstat9p"
	case TTwstat9P:
		return "Twstat9p"
	case TRwstat:
		return "Rwstat"

	case TRerror:
		return "Rerror"
	case TTopen:
		return "Ropen"
	case TTcreate:
		return "Tcreate"
	case TTreadV:
		return "TreadV"
	case TTwriteV:
		return "TwriteV"
	case TRstat:
		return "Rstat"
	case TTwstat:
		return "Tstat"
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
	default:
		return "Tunknown"
	}
}
