package ninep

//
// Go structures for 9P based on the wire format in Linux's 9p net/9p,
// include/net/9p, and various Go 9p implementations.
//

import (
	"fmt"
)

type Tsize uint32
type Ttag uint16
type Tfid uint32
type Tiounit uint32
type Tperm uint32
type Toffset uint64
type Tlength uint64
type Tgid uint32

// NoTag is the tag for Tversion and Rversion requests.
const NoTag Ttag = ^Ttag(0)

// NoFid is a reserved fid used in a Tattach request for the afid
// field, that indicates that the client does not wish to authenticate
// this session.
const NoFid Tfid = ^Tfid(0)

type Tpath uint64
type Qtype uint8
type TQversion uint32

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

// A Qid is the server's unique identification for the file being
// accessed: two files on the same server hierarchy are the same if
// and only if their qids are the same.
type Tqid struct {
	Type    Qtype
	Version TQversion
	Path    Tpath
}

func MakeQid(t Qtype, v TQversion, p Tpath) Tqid {
	return Tqid{t, v, p}
}

type Tmode uint8

// Flags for the mode field in Topen and Tcreate messages
const (
	OREAD    Tmode = 0    // read-only
	OWRITE   Tmode = 0x01 // write-only
	ORDWR    Tmode = 0x02 // read-write
	OEXEC    Tmode = 0x03 // execute (implies OREAD)
	OTRUNC   Tmode = 0x10 // or truncate file first
	OCEXEC   Tmode = 0x20 // or close on exec
	ORCLOSE  Tmode = 0x40 // remove on close
	OAPPEND  Tmode = 0x80 // append
	OVERSION Tmode = 0x83 // ulambda extension hack (overloads OAPPEND|OEXEC)
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
	DMTMP    Tperm = 0x04000000 // non-backed-up file

	DMREAD  = 0x4 // mode bit for read permission
	DMWRITE = 0x2 // mode bit for write permission
	DMEXEC  = 0x1 // mode bit for execute permission

	// 9P2000.u extensions
	DMSYMLINK   Tperm = 0x02000000
	DMLINK      Tperm = 0x01000000
	DMDEVICE    Tperm = 0x00800000
	DMNAMEDPIPE Tperm = 0x00200000
	DMSOCKET    Tperm = 0x00100000
	DMSETUID    Tperm = 0x00080000
	DMSETGID    Tperm = 0x00040000
	DMSETVTX    Tperm = 0x00010000
)

const (
	QTYPESHIFT = 24
	TYPESHIFT  = 16
)

func (p Tperm) IsDir() bool       { return p&DMDIR == DMDIR }
func (p Tperm) IsSymlink() bool   { return p&DMSYMLINK == DMSYMLINK }
func (p Tperm) IsDevice() bool    { return p&DMDEVICE == DMDEVICE }
func (p Tperm) IsPipe() bool      { return p&DMNAMEDPIPE == DMNAMEDPIPE }
func (p Tperm) IsEphemeral() bool { return p&DMTMP == DMTMP }
func (p Tperm) IsFile() bool      { return (p >> TYPESHIFT) == 0 }

func (p Tperm) String() string {
	qt := Qtype(p >> QTYPESHIFT)
	return fmt.Sprintf("qt %v p %x", qt, uint8(p&0xFF))
}

type Tfcall uint8

const (
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
	TTreadv
	TTwritev
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
	case TTreadv:
		return "Treadv"
	case TTwritev:
		return "Twritev"
	default:
		return "Tunknown"
	}
}

type Tmsg interface {
	Type() Tfcall
}

type Fcall struct {
	Type Tfcall
	Tag  Ttag
	Msg  Tmsg
}

type Tversion struct {
	Msize   Tsize
	Version string
}

type Rversion struct {
	Msize   Tsize
	Version string
}

type Tauth struct {
	Afid   Tfid
	Unames []string
	Anames []string
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

type Rattach struct {
	Qid Tqid
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

type Treadv struct {
	Fid     Tfid
	Offset  Toffset
	Count   Tsize
	Version TQversion
}

type Rread struct {
	Data []byte
}

type Twrite struct {
	Fid    Tfid
	Offset Toffset
	Data   []byte
}

type Twritev struct {
	Fid     Tfid
	Offset  Toffset
	Data    []byte
	Version TQversion
}

func (tw Twrite) String() string {
	return fmt.Sprintf("Twrite(%v off %v cnt %v", tw.Fid, tw.Offset, len(tw.Data))
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

type Rremove struct {
}

type Tstat struct {
	Fid Tfid
}

type Stat struct {
	Type   uint16
	Dev    uint32
	Qid    Tqid
	Mode   Tperm
	Atime  uint32  // last access time in seconds
	Mtime  uint32  // last modified time in seconds
	Length Tlength // file length in bytes
	Name   string  // file name
	Uid    string  // owner name
	Gid    string  // group name
	Muid   string  // name of the last user that modified the file

}

func (s Stat) String() string {
	return fmt.Sprintf("stat(%v mode=%v atime=%v mtime=%v length=%v name=%v uid=%v gid=%v muid=%v)",
		s.Qid, s.Mode, s.Atime, s.Mtime, s.Length, s.Name, s.Uid, s.Gid, s.Muid)
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

func (Tversion) Type() Tfcall { return TTversion }
func (Rversion) Type() Tfcall { return TRversion }
func (Tauth) Type() Tfcall    { return TTauth }
func (Rauth) Type() Tfcall    { return TRauth }
func (Tflush) Type() Tfcall   { return TTflush }
func (Rflush) Type() Tfcall   { return TRflush }
func (Tattach) Type() Tfcall  { return TTattach }
func (Rattach) Type() Tfcall  { return TRattach }
func (Rerror) Type() Tfcall   { return TRerror }
func (Twalk) Type() Tfcall    { return TTwalk }
func (Rwalk) Type() Tfcall    { return TRwalk }
func (Topen) Type() Tfcall    { return TTopen }
func (Ropen) Type() Tfcall    { return TRopen }
func (Tcreate) Type() Tfcall  { return TTcreate }
func (Rcreate) Type() Tfcall  { return TRcreate }
func (Tread) Type() Tfcall    { return TTread }
func (Treadv) Type() Tfcall   { return TTreadv }
func (Rread) Type() Tfcall    { return TRread }
func (Twrite) Type() Tfcall   { return TTwrite }
func (Twritev) Type() Tfcall  { return TTwritev }
func (Rwrite) Type() Tfcall   { return TRwrite }
func (Tclunk) Type() Tfcall   { return TTclunk }
func (Rclunk) Type() Tfcall   { return TRclunk }
func (Tremove) Type() Tfcall  { return TTremove }
func (Rremove) Type() Tfcall  { return TRremove }
func (Tstat) Type() Tfcall    { return TTstat }
func (Rstat) Type() Tfcall    { return TRstat }
func (Twstat) Type() Tfcall   { return TTwstat }
func (Rwstat) Type() Tfcall   { return TRwstat }
