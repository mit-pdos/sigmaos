package sessp

import (
	"fmt"
	"log"
	"runtime/debug"
	"strconv"
	"strings"
	"sync/atomic"
)

type Tfcall uint8
type Ttag uint16
type Tsize uint32
type Tpath uint64

type Tsession uint64
type Tseqno uint64
type Tclient uint64
type Tepoch uint64

// NoTag is the tag for Tversion and Rversion requests.
const NoTag Ttag = ^Ttag(0)

func (p Tpath) String() string {
	return strconv.FormatUint(uint64(p), 16)
}

func String2Path(path string) (Tpath, error) {
	p, err := strconv.ParseUint(path, 16, 64)
	if err != nil {
		return Tpath(p), err
	}
	return Tpath(p), nil
}

const NoPath Tpath = ^Tpath(0)

// NoSeqno signifies the fcall came from a wire-compatible peer
const NoSeqno Tseqno = ^Tseqno(0)

// Atomically increment pointer and return result
func (n *Tseqno) Next() Tseqno {
	next := atomic.AddUint64((*uint64)(n), 1)
	return Tseqno(next)
}

// NoSession signifies the fcall came from a wire-compatible peer
const NoSession Tsession = ^Tsession(0)

func (s Tsession) String() string {
	return strconv.FormatUint(uint64(s), 16)
}

const NoEpoch Tepoch = ^Tepoch(0)

func (e Tepoch) String() string {
	return strconv.FormatUint(uint64(e), 16)
}

func String2Epoch(epoch string) (Tepoch, error) {
	e, err := strconv.ParseUint(epoch, 16, 64)
	if err != nil {
		return Tepoch(0), err
	}
	return Tepoch(e), nil
}

type Tmsg interface {
	Type() Tfcall
}

type SessReply struct {
	Fcm          *FcallMsg
	MarshaledFcm []byte
}

func MakeSessReply(fcm *FcallMsg, b []byte) *SessReply {
	return &SessReply{
		fcm,
		b,
	}
}

type FcallMsg struct {
	Fc   *Fcall
	Msg  Tmsg
	Data []byte
}

func (fcm *FcallMsg) Session() Tsession {
	return Tsession(fcm.Fc.Session)
}

func (fcm *FcallMsg) Client() Tclient {
	return Tclient(fcm.Fc.Client)
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

func (fcm *FcallMsg) Tag() Ttag {
	return Ttag(fcm.Fc.Tag)
}

func MakeFenceNull() *Tfence {
	return &Tfence{Fenceid: &Tfenceid{}}
}

func MakeFcallMsgNull() *FcallMsg {
	fc := &Fcall{Received: &Tinterval{}, Fence: MakeFenceNull()}
	return &FcallMsg{fc, nil, nil}
}

func (fi *Tfenceid) Tpath() Tpath {
	return Tpath(fi.Path)
}

func (f *Tfence) Tepoch() Tepoch {
	return Tepoch(f.Epoch)
}

func MakeFcallMsg(msg Tmsg, data []byte, cli Tclient, sess Tsession, seqno *Tseqno, rcv *Tinterval, f *Tfence) *FcallMsg {
	if rcv == nil {
		rcv = &Tinterval{}
	}
	fcall := &Fcall{
		Type:     uint32(msg.Type()),
		Tag:      0,
		Client:   uint64(cli),
		Session:  uint64(sess),
		Received: rcv,
		Fence:    f,
	}
	if seqno != nil {
		fcall.Seqno = uint64(seqno.Next())
	}
	return &FcallMsg{fcall, msg, data}
}

func MakeFcallMsgReply(req *FcallMsg, reply Tmsg) *FcallMsg {
	fm := MakeFcallMsg(reply, nil, Tclient(req.Fc.Client), Tsession(req.Fc.Session), nil, nil, MakeFenceNull())
	fm.Fc.Seqno = req.Fc.Seqno
	fm.Fc.Received = req.Fc.Received
	fm.Fc.Tag = req.Fc.Tag
	return fm
}

func (fm *FcallMsg) String() string {
	return fmt.Sprintf("%v t %v s %v seq %v recv %v msg %v f %v", fm.Msg.Type(), fm.Fc.Tag, fm.Fc.Session, fm.Fc.Seqno, fm.Fc.Received, fm.Msg, fm.Fc.Fence)
}

func (fm *FcallMsg) GetType() Tfcall {
	return Tfcall(fm.Fc.Type)
}

func (fm *FcallMsg) GetMsg() Tmsg {
	return fm.Msg
}

func MkInterval(start, end uint64) *Tinterval {
	return &Tinterval{
		Start: start,
		End:   end,
	}
}

func (iv *Tinterval) Size() Tsize {
	return Tsize(iv.End - iv.Start)
}

// XXX should atoi be uint64?
func (iv *Tinterval) Unmarshal(s string) {
	idxs := strings.Split(s[1:len(s)-1], ", ")
	start, err := strconv.Atoi(idxs[0])
	if err != nil {
		debug.PrintStack()
		log.Fatalf("FATAL unmarshal interval: %v", err)
	}
	iv.Start = uint64(start)
	end, err := strconv.Atoi(idxs[1])
	if err != nil {
		debug.PrintStack()
		log.Fatalf("FATAL unmarshal interval: %v", err)
	}
	iv.End = uint64(end)
}

func (iv *Tinterval) Marshal() string {
	return fmt.Sprintf("[%d, %d)", iv.Start, iv.End)
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
