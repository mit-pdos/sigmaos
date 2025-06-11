package proto

import (
	"fmt"
	"strconv"
	"sync/atomic"
)

type Tfcall uint8
type Ttag uint64

type Tsession uint64
type Tseqno uint64
type Tseqcntr = atomic.Uint64

// NoTag is the tag for Tversion and Rversion requests.
const NoTag Ttag = ^Ttag(0)

// NoSeqno signifies the fcall came from a wire-compatible peer
const NoSeqno Tseqno = ^Tseqno(0)

// Atomically increment pointer and return result
func NextSeqno(sc *Tseqcntr) Tseqno {
	next := sc.Add(1)
	return Tseqno(next)
}

// NoSession signifies the fcall came from a wire-compatible peer
const NoSession Tsession = ^Tsession(0)

func (s Tsession) String() string {
	return strconv.FormatUint(uint64(s), 16)
}

type Tframe []byte
type IoVec []Tframe

func NewIoVec(fs [][]byte) IoVec {
	iov := make(IoVec, len(fs))
	for i := 0; i < len(fs); i++ {
		iov[i] = fs[i]
	}
	return iov
}

func (iov IoVec) String() string {
	s := fmt.Sprintf("len %d [", len(iov))
	for _, f := range iov {
		s += fmt.Sprintf("%d,", len(f))
	}
	s += fmt.Sprintf("]")
	return s
}

type Tmsg interface {
	Type() Tfcall
}

type FcallMsg struct {
	Fc  *Fcall
	Msg Tmsg
	Iov IoVec
}

func (fcm *FcallMsg) Session() Tsession {
	return Tsession(fcm.Fc.Session)
}

func (fcm *FcallMsg) Type() Tfcall {
	return Tfcall(fcm.Fc.Type)
}

func (fc *Fcall) Tseqno() Tseqno {
	return Tseqno(fc.Seqno)
}

func (fcm *FcallMsg) Seqno() Tseqno {
	return fcm.Fc.Tseqno()
}

// Use seqno as tag
func (fcm *FcallMsg) Tag() Ttag {
	return Ttag(fcm.Fc.Seqno)
}

func NewFcallMsgNull() *FcallMsg {
	fc := &Fcall{}
	return &FcallMsg{fc, nil, nil}
}

func NewFcallMsg(msg Tmsg, iov IoVec, sess Tsession, seqcntr *Tseqcntr) *FcallMsg {
	fcall := &Fcall{
		Type:    uint32(msg.Type()),
		Session: uint64(sess),
	}
	if seqcntr != nil {
		fcall.Seqno = uint64(NextSeqno(seqcntr))
	}
	return &FcallMsg{fcall, msg, iov}
}

func NewFcallMsgReply(req *FcallMsg, reply Tmsg) *FcallMsg {
	fm := NewFcallMsg(reply, nil, Tsession(req.Fc.Session), nil)
	fm.Fc.Seqno = req.Fc.Seqno
	return fm
}

func (fm *FcallMsg) String() string {
	var typ Tfcall
	var msg Tmsg
	var seqno Tseqno
	var sess Tsession
	var l int
	var nvec int
	if fm.Msg != nil {
		typ = fm.Msg.Type()
		msg = fm.Msg
	}
	if fm.Fc != nil {
		seqno = Tseqno(fm.Fc.Seqno)
		sess = Tsession(fm.Fc.Session)
		l = int(fm.Fc.Len)
		nvec = int(fm.Fc.Nvec)
	}
	return fmt.Sprintf("{ %v seq %v sid %v len %d nvec %d msg %v iov %d }", typ, seqno, sess, l, nvec, msg, len(fm.Iov))
}

func (fm *FcallMsg) GetType() Tfcall {
	return Tfcall(fm.Fc.Type)
}

func (fm *FcallMsg) GetMsg() Tmsg {
	return fm.Msg
}

// A partially marshaled message to push the cost of marshaling Fcm
// out of demux clnt.
type PartMarshaledMsg struct {
	Fcm          *FcallMsg
	MarshaledFcm []byte
}

func (pmfc *PartMarshaledMsg) Tag() Ttag {
	return pmfc.Fcm.Tag()
}

func (pmfc *PartMarshaledMsg) String() string {
	var typ Tfcall
	if pmfc.Fcm != nil {
		typ = Tfcall(pmfc.Fcm.Type())
	}
	return fmt.Sprintf("&{ type:%v fc:%v }", typ, pmfc.Fcm)
}

const (

	//
	// 9P
	//

	TTversion Tfcall = iota + 100
	TRversion
	TTauth
	TRauth
	TTattach9P
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
	TTremove9P
	TRremove
	TTstat
	TRstat9P
	TTwstat9P
	TRwstat

	//
	// SigmaP
	//
	TTattach
	TRerror
	TTopen
	TTcreate
	TTreadF
	TRread
	TTwriteF
	TTwatch
	TRwatch
	TRstat
	TTwstat
	TTrenameat
	TRrenameat
	TTremove
	TTremovefile
	TTgetfile
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
	case TTattach9P:
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
	case TRread9P:
		return "Rread9P"
	case TTwrite:
		return "Twrite9P"
	case TRwrite:
		return "Rwrite9P"
	case TTclunk:
		return "Tclunk"
	case TRclunk:
		return "Rclunk"
	case TTremove9P:
		return "Tremove9P"
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

	case TTattach:
		return "Tattach"
	case TRerror:
		return "Rerror"
	case TTopen:
		return "Topen"
	case TTcreate:
		return "Tcreate"
	case TTreadF:
		return "TreadF"
	case TTwriteF:
		return "TwriteF"
	case TRstat:
		return "Rstat"
	case TTremove:
		return "Tremove"
	case TTwstat:
		return "Tstat"
	case TTwatch:
		return "Twatch"
	case TRwatch:
		return "Rwatch"
	case TTrenameat:
		return "Trenameat"
	case TRrenameat:
		return "Rrenameat"
	case TTremovefile:
		return "Tremovefile"
	case TTgetfile:
		return "Tgetfile"
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
