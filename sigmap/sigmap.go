package sigmap

//
// Go structures for sigmap protocol, which is based on 9P.
//

import (
	"fmt"
	"log"
	"runtime/debug"
	"strconv"
	"strings"
	"sync/atomic"

	"sigmaos/fcall"
)

type Tsize uint32
type Ttag uint16
type Tfid uint32
type Tiounit uint32
type Tperm uint32
type Toffset uint64
type Tlength uint64
type Tgid uint32

func (fid Tfid) String() string {
	if fid == NoFid {
		return "-1"
	}
	return fmt.Sprintf("fid %d", fid)
}

//
// Augmentated types for sigmaOS
//

type Tseqno uint64

// NoSeqno signifies the fcall came from a wire-compatible peer
const NoSeqno Tseqno = ^Tseqno(0)

// Atomically increment pointer and return result
func (n *Tseqno) Next() Tseqno {
	next := atomic.AddUint64((*uint64)(n), 1)
	return Tseqno(next)
}

type Tepoch uint64

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

//
//  End augmentated types
//

// NoTag is the tag for Tversion and Rversion requests.
const NoTag Ttag = ^Ttag(0)

// NoFid is a reserved fid used in a Tattach request for the afid
// field, that indicates that the client does not wish to authenticate
// this session.
const NoFid Tfid = ^Tfid(0)
const NoOffset Toffset = ^Toffset(0)

// If need more than MaxGetSet, use Open/Read/Close interface
const MAXGETSET Tsize = 1_000_000

type Tpath uint64

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

type Qtype uint32
type TQversion uint32

const NoPath Tpath = ^Tpath(0)
const NoV TQversion = ^TQversion(0)

func VEq(v1, v2 TQversion) bool {
	return v1 == NoV || v1 == v2
}

// A Qid's type field represents the type of a file, the high 8 bits of
// the file's permission.
const (
	QTDIR     Qtype = 0x80 // directories
	QTAPPEND  Qtype = 0x40 // append only files
	QTEXCL    Qtype = 0x20 // exclusive use files
	QTMOUNT   Qtype = 0x10 // mounted channel
	QTAUTH    Qtype = 0x08 // authentication file (afid)
	QTTMP     Qtype = 0x04 // non-backed-up file
	QTSYMLINK Qtype = 0x02
	QTFILE    Qtype = 0x00
)

func (qt Qtype) String() string {
	s := ""
	if qt&QTDIR == QTDIR {
		s += "d"
	}
	if qt&QTAPPEND == QTAPPEND {
		s += "a"
	}
	if qt&QTEXCL == QTEXCL {
		s += "e"
	}
	if qt&QTMOUNT == QTMOUNT {
		s += "m"
	}
	if qt&QTAUTH == QTAUTH {
		s += "auth"
	}
	if qt&QTTMP == QTTMP {
		s += "t"
	}
	if qt&QTSYMLINK == QTSYMLINK {
		s += "s"
	}
	if s == "" {
		s = "f"
	}
	return s
}

func MakeQid(t Qtype, v TQversion, p Tpath) *Tqid {
	return &Tqid{Type: uint32(t), Version: uint32(v), Path: uint64(p)}
}

func MakeQidPerm(perm Tperm, v TQversion, p Tpath) *Tqid {
	return MakeQid(Qtype(perm>>QTYPESHIFT), v, p)
}

func (qid *Tqid) Tversion() TQversion {
	return TQversion(qid.Version)
}

func (qid *Tqid) Tpath() Tpath {
	return Tpath(qid.Path)
}

func (qid *Tqid) Ttype() Qtype {
	return Qtype(qid.Type)
}

type Tmode uint8

// Flags for the mode field in Topen and Tcreate messages
const (
	OREAD   Tmode = 0    // read-only
	OWRITE  Tmode = 0x01 // write-only
	ORDWR   Tmode = 0x02 // read-write
	OEXEC   Tmode = 0x03 // execute (implies OREAD)
	OTRUNC  Tmode = 0x10 // or truncate file first
	OCEXEC  Tmode = 0x20 // or close on exec
	ORCLOSE Tmode = 0x40 // remove on close
	OAPPEND Tmode = 0x80 // append

	// sigmaP extension: a client uses OWATCH to block at the
	// server until a file/directiory is create or removed, or a
	// directory changes.  OWATCH with Tcreate will block if the
	// file exists and a remove() will unblock that create.
	// OWATCH with Open() and a closure will invoke the closure
	// when a client creates or removes the file.  OWATCH on open
	// for a directory and a closure will invoke the closure if
	// the directory changes.
	OWATCH Tmode = OCEXEC // overleads OEXEC; maybe ORCLOSe better?
)

func (m Tmode) String() string {
	return fmt.Sprintf("m %x", uint8(m&0xFF))
}

// Permissions
const (
	DMDIR    Tperm = 0x80000000 // directory
	DMAPPEND Tperm = 0x40000000 // append only file
	DMEXCL   Tperm = 0x20000000 // exclusive use file
	DMMOUNT  Tperm = 0x10000000 // mounted channel
	DMAUTH   Tperm = 0x08000000 // authentication file

	// DMTMP is ephemeral in sigmaP
	DMTMP Tperm = 0x04000000 // non-backed-up file

	DMREAD  = 0x4 // mode bit for read permission
	DMWRITE = 0x2 // mode bit for write permission
	DMEXEC  = 0x1 // mode bit for execute permission

	// 9P2000.u extensions
	// A few are used by ulambda, but not supported in driver/proxy,
	// so ulambda mounts on Linux without these extensions.
	DMSYMLINK   Tperm = 0x02000000
	DMLINK      Tperm = 0x01000000
	DMDEVICE    Tperm = 0x00800000
	DMREPL      Tperm = 0x00400000
	DMNAMEDPIPE Tperm = 0x00200000
	DMSOCKET    Tperm = 0x00100000
	DMSETUID    Tperm = 0x00080000
	DMSETGID    Tperm = 0x00040000
	DMSETVTX    Tperm = 0x00010000
)

const (
	QTYPESHIFT = 24
	TYPESHIFT  = 16
	TYPEMASK   = 0xFF
)

func (p Tperm) IsDir() bool        { return p&DMDIR == DMDIR }
func (p Tperm) IsSymlink() bool    { return p&DMSYMLINK == DMSYMLINK }
func (p Tperm) IsReplicated() bool { return p&DMREPL == DMREPL }
func (p Tperm) IsDevice() bool     { return p&DMDEVICE == DMDEVICE }
func (p Tperm) IsPipe() bool       { return p&DMNAMEDPIPE == DMNAMEDPIPE }
func (p Tperm) IsEphemeral() bool  { return p&DMTMP == DMTMP }
func (p Tperm) IsFile() bool       { return (p>>QTYPESHIFT)&0xFF == 0 }

func (p Tperm) String() string {
	qt := Qtype(p >> QTYPESHIFT)
	return fmt.Sprintf("qt %v qp %x", qt, uint8(p&TYPEMASK))
}

type Tmsg interface {
	Type() fcall.Tfcall
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

type FcallMsg struct {
	Fc  *Fcall
	Msg Tmsg
}

func (fcm *FcallMsg) Session() fcall.Tsession {
	return fcall.Tsession(fcm.Fc.Session)
}

func (fcm *FcallMsg) Client() fcall.Tclient {
	return fcall.Tclient(fcm.Fc.Client)
}

func (fcm *FcallMsg) Type() fcall.Tfcall {
	return fcall.Tfcall(fcm.Fc.Type)
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
	return &FcallMsg{fc, nil}
}

func (fi *Tfenceid) Tpath() Tpath {
	return Tpath(fi.Path)
}

func (f *Tfence) Tepoch() Tepoch {
	return Tepoch(f.Epoch)
}

func MakeFcallMsg(msg Tmsg, cli fcall.Tclient, sess fcall.Tsession, seqno *Tseqno, rcv *Tinterval, f *Tfence) *FcallMsg {
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
	return &FcallMsg{fcall, msg}
}

func MakeFcallMsgReply(req *FcallMsg, reply Tmsg) *FcallMsg {
	fm := MakeFcallMsg(reply, fcall.Tclient(req.Fc.Client), fcall.Tsession(req.Fc.Session), nil, nil, MakeFenceNull())
	fm.Fc.Seqno = req.Fc.Seqno
	fm.Fc.Received = req.Fc.Received
	fm.Fc.Tag = req.Fc.Tag
	return fm
}

func (fm *FcallMsg) String() string {
	return fmt.Sprintf("%v t %v s %v seq %v recv %v msg %v f %v", fm.Msg.Type(), fm.Fc.Tag, fm.Fc.Session, fm.Fc.Seqno, fm.Fc.Received, fm.Msg, fm.Fc.Fence)
}

func (fm *FcallMsg) GetType() fcall.Tfcall {
	return fcall.Tfcall(fm.Fc.Type)
}

func (fm *FcallMsg) GetMsg() Tmsg {
	return fm.Msg
}

type Tversion struct {
	Msize   Tsize
	Version string
}

func (m Tversion) String() string {
	return fmt.Sprintf("{msize %v version %v}", m.Msize, m.Version)
}

type Rversion struct {
	Msize   Tsize
	Version string
}

func (m Rversion) String() string {
	return fmt.Sprintf("{msize %v version %v}", m.Msize, m.Version)
}

type Tauth struct {
	Afid   Tfid
	Unames []string
	Anames []string
}

func (m Tauth) String() string {
	return fmt.Sprintf("{afid %v u %v a %v}", m.Afid, m.Unames, m.Anames)
}

type Rauth struct {
	Aqid Tqid
}

type Tattach struct {
	Fid   Tfid
	Afid  Tfid
	Uname string
	Aname string
}

func (m Tattach) String() string {
	return fmt.Sprintf("{%v a %v u %v a '%v'}", m.Fid, m.Afid, m.Uname, m.Aname)
}

func MkRerror(err *fcall.Err) *Rerror {
	return &Rerror{err.Error()}
}

type Rerror struct {
	Ename string
}

type Tflush struct {
	Oldtag Ttag
}

type Rflush struct {
}

type Twalk struct {
	Fid    Tfid
	NewFid Tfid
	Wnames []string
}

type Rwalk struct {
	Qids []Tqid
}

type Topen struct {
	Fid  Tfid
	Mode Tmode
}

type Twatch struct {
	Fid Tfid
}

type Ropen struct {
	Qid    Tqid
	Iounit Tiounit
}

type Tcreate struct {
	Fid  Tfid
	Name string
	Perm Tperm
	Mode Tmode
}

type Rcreate struct {
	Qid    Tqid
	Iounit Tiounit
}

type Tread struct {
	Fid    Tfid
	Offset Toffset
	Count  Tsize
}

type TreadV struct {
	Fid     Tfid
	Offset  Toffset
	Count   Tsize
	Version TQversion
}

type Rread struct {
	Data []byte
}

func (rr Rread) String() string {
	return fmt.Sprintf("{len %d}", len(rr.Data))
}

type Twrite struct {
	Fid    Tfid
	Offset Toffset
	Data   []byte // Data must be last
}

func (tw Twrite) String() string {
	return fmt.Sprintf("{%v off %v len %d}", tw.Fid, tw.Offset, len(tw.Data))
}

type TwriteV struct {
	Fid     Tfid
	Offset  Toffset
	Version TQversion
	Data    []byte // Data must be last
}

func (tw TwriteV) String() string {
	return fmt.Sprintf("{%v off %v len %d v %v}", tw.Fid, tw.Offset, len(tw.Data), tw.Version)
}

type Rwrite struct {
	Count Tsize
}

type Tclunk struct {
	Fid Tfid
}

type Rclunk struct {
}

type Tremove struct {
	Fid Tfid
}

type Tremovefile struct {
	Fid     Tfid
	Wnames  []string
	Resolve bool
}

type Rremove struct {
}

type Tstat struct {
	Fid Tfid
}

func MkStatNull() *Stat {
	st := &Stat{}
	st.Qid = MakeQid(0, 0, 0)
	return st
}

func MkStat(qid *Tqid, perm Tperm, mtime uint32, name, owner string) *Stat {
	st := &Stat{
		Type:  0, // XXX
		Qid:   qid,
		Mode:  uint32(perm),
		Mtime: mtime,
		Atime: 0,
		Name:  name,
		Uid:   owner,
		Gid:   owner,
		Muid:  "",
	}
	return st

}

func (st *Stat) Tlength() Tlength {
	return Tlength(st.Length)
}

func (st *Stat) Tmode() Tperm {
	return Tperm(st.Mode)
}

type Rstat struct {
	Size uint16 // extra Size, see stat(5)
	Stat Stat
}

type Twstat struct {
	Fid  Tfid
	Size uint16 // extra Size, see stat(5)
	Stat Stat
}

type Rwstat struct{}

type Trenameat struct {
	OldFid  Tfid
	OldName string
	NewFid  Tfid
	NewName string
}

type Rrenameat struct{}

type Tgetfile struct {
	Fid     Tfid
	Mode    Tmode
	Offset  Toffset
	Count   Tsize
	Wnames  []string
	Resolve bool
}

func (m Tgetfile) String() string {
	return fmt.Sprintf("{%v off %v p %v cnt %v}", m.Fid, m.Offset, m.Wnames, m.Count)
}

type Rgetfile struct {
	Data []byte
}

func (m Rgetfile) String() string {
	return fmt.Sprintf("{len %v}", len(m.Data))
}

type Tsetfile struct {
	Fid     Tfid
	Mode    Tmode
	Offset  Toffset
	Wnames  []string
	Resolve bool
	Data    []byte // Data must be last
}

func (m Tsetfile) String() string {
	return fmt.Sprintf("{%v off %v p %v r %v len %v}", m.Fid, m.Offset, m.Wnames, m.Resolve, len(m.Data))
}

type Tputfile struct {
	Fid    Tfid
	Mode   Tmode
	Perm   Tperm
	Offset Toffset
	Wnames []string
	Data   []byte // Data must be last
}

func (m Tputfile) String() string {
	return fmt.Sprintf("{%v %v p %v off %v p %v len %v}", m.Fid, m.Mode, m.Perm, m.Offset, m.Wnames, len(m.Data))
}

type Tdetach struct {
	PropId uint32 // ID of the server proposing detach.
	LeadId uint32 // ID of the leader when change was proposed (filled in later).
}

type Rdetach struct {
}

type Theartbeat struct {
	Sids []fcall.Tsession // List of sessions in this heartbeat.
}

type Rheartbeat struct {
	Sids []fcall.Tsession // List of sessions in this heartbeat.
}

// type Twriteread struct {
// 	Fid  Tfid
// 	Data []byte // Data must be last
// }

// type Rwriteread struct {
// 	Data []byte // Data must be last
// }

func (Tversion) Type() fcall.Tfcall { return fcall.TTversion }
func (Rversion) Type() fcall.Tfcall { return fcall.TRversion }
func (Tauth) Type() fcall.Tfcall    { return fcall.TTauth }
func (Rauth) Type() fcall.Tfcall    { return fcall.TRauth }
func (Tflush) Type() fcall.Tfcall   { return fcall.TTflush }
func (Rflush) Type() fcall.Tfcall   { return fcall.TRflush }
func (Tattach) Type() fcall.Tfcall  { return fcall.TTattach }
func (Rattach) Type() fcall.Tfcall  { return fcall.TRattach }
func (Rerror) Type() fcall.Tfcall   { return fcall.TRerror }
func (Twalk) Type() fcall.Tfcall    { return fcall.TTwalk }
func (Rwalk) Type() fcall.Tfcall    { return fcall.TRwalk }
func (Topen) Type() fcall.Tfcall    { return fcall.TTopen }
func (Twatch) Type() fcall.Tfcall   { return fcall.TTwatch }
func (Ropen) Type() fcall.Tfcall    { return fcall.TRopen }
func (Tcreate) Type() fcall.Tfcall  { return fcall.TTcreate }
func (Rcreate) Type() fcall.Tfcall  { return fcall.TRcreate }
func (Tread) Type() fcall.Tfcall    { return fcall.TTread }
func (Rread) Type() fcall.Tfcall    { return fcall.TRread }
func (Twrite) Type() fcall.Tfcall   { return fcall.TTwrite }
func (Rwrite) Type() fcall.Tfcall   { return fcall.TRwrite }
func (Tclunk) Type() fcall.Tfcall   { return fcall.TTclunk }
func (Rclunk) Type() fcall.Tfcall   { return fcall.TRclunk }
func (Tremove) Type() fcall.Tfcall  { return fcall.TTremove }
func (Rremove) Type() fcall.Tfcall  { return fcall.TRremove }
func (Tstat) Type() fcall.Tfcall    { return fcall.TTstat }
func (Rstat) Type() fcall.Tfcall    { return fcall.TRstat }
func (Twstat) Type() fcall.Tfcall   { return fcall.TTwstat }
func (Rwstat) Type() fcall.Tfcall   { return fcall.TRwstat }

//
// sigmaP
//

func (TreadV) Type() fcall.Tfcall      { return fcall.TTreadV }
func (TwriteV) Type() fcall.Tfcall     { return fcall.TTwriteV }
func (Trenameat) Type() fcall.Tfcall   { return fcall.TTrenameat }
func (Rrenameat) Type() fcall.Tfcall   { return fcall.TRrenameat }
func (Tremovefile) Type() fcall.Tfcall { return fcall.TTremovefile }
func (Tgetfile) Type() fcall.Tfcall    { return fcall.TTgetfile }
func (Rgetfile) Type() fcall.Tfcall    { return fcall.TRgetfile }
func (Tsetfile) Type() fcall.Tfcall    { return fcall.TTsetfile }
func (Tputfile) Type() fcall.Tfcall    { return fcall.TTputfile }
func (Tdetach) Type() fcall.Tfcall     { return fcall.TTdetach }
func (Rdetach) Type() fcall.Tfcall     { return fcall.TRdetach }
func (Theartbeat) Type() fcall.Tfcall  { return fcall.TTheartbeat }
func (Rheartbeat) Type() fcall.Tfcall  { return fcall.TRheartbeat }
func (Twriteread) Type() fcall.Tfcall  { return fcall.TTwriteread }
func (Rwriteread) Type() fcall.Tfcall  { return fcall.TRwriteread }
