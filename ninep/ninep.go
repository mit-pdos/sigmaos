package ninep

//
// Go structures for 9P based on the wire format in Linux's 9p net/9p,
// include/net/9p, and
// https://github.com/chaos/diod/blob/master/protocol.md
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
	switch qt {
	case QTDIR:
		return "d"
	case QTAPPEND:
		return "a"
	case QTEXCL:
		return "e"
	case QTMOUNT:
		return "m"
	case QTAUTH:
		return "auth"
	case QTTMP:
		return "tmp"
	case QTFILE:
		return "f"
	}
	return "unknown"
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
	OREAD   Tmode = 0    // read-only
	OWRITE  Tmode = 0x01 // write-only
	ORDWR   Tmode = 0x02 // read-write
	OEXEC   Tmode = 0x03 // execute (implies OREAD)
	OTRUNC  Tmode = 0x10 // or truncate file first
	OCEXEC  Tmode = 0x20 // or close on exec
	ORCLOSE Tmode = 0x40 // remove on close
	OAPPEND Tmode = 0x80 // append
)

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

func IsDir(t Tperm) bool     { return t&DMDIR == DMDIR }
func IsSymlink(t Tperm) bool { return t&DMSYMLINK == DMSYMLINK }
func IsDevice(t Tperm) bool  { return t&DMDEVICE == DMDEVICE }
func IsPipe(t Tperm) bool    { return t&DMNAMEDPIPE == DMNAMEDPIPE }
func IsFile(t Tperm) bool    { return (t >> TYPESHIFT) == 0 }

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

type Rread struct {
	Data []byte
}

type Twrite struct {
	Fid    Tfid
	Offset Toffset
	Data   []byte
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
	Stat Stat
}

type Twstat struct {
	Fid  Tfid
	Stat []byte
}

type Rwstat struct {
}

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
func (Rread) Type() Tfcall    { return TRread }
func (Twrite) Type() Tfcall   { return TTwrite }
func (Rwrite) Type() Tfcall   { return TRwrite }
func (Tclunk) Type() Tfcall   { return TTclunk }
func (Rclunk) Type() Tfcall   { return TRclunk }
func (Tremove) Type() Tfcall  { return TTremove }
func (Rremove) Type() Tfcall  { return TRremove }
func (Tstat) Type() Tfcall    { return TTstat }
func (Rstat) Type() Tfcall    { return TRstat }

//func (Twstat) Type() Tfcall { return TTwstat }
//func (Rwstat) Type() Tfcall { return TRwstat }

//
// Extensions or new transactions
//

type Dirent struct {
	Qid    Tqid
	Offset Toffset
	Type   Tperm
	Name   string
}

type Tmkdir struct {
	Dfid Tfid
	Name string
	Mode Tmode
	Gid  Tgid
}

type Rmkdir struct {
	Qid Tqid
}

type Treaddir struct {
	Fid    Tfid
	Offset Toffset
	Count  Tsize
}

type Rreaddir struct {
	Data []byte
}

type Tsymlink struct {
	Fid    Tfid
	Name   string
	Symtgt string
	Gid    Tgid
}

type Rsymlink struct {
	Qid Tqid
}

type Treadlink struct {
	Fid Tfid
}

type Rreadlink struct {
	Target string
}

type Tmkpipe struct {
	Dfid Tfid
	Name string
	Mode Tmode
	Gid  Tgid
}

type Rmkpipe struct {
	Qid Tqid
}
