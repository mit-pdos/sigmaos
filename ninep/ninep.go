// The ninep package has Go structures for 9P based on the wire format
// in Linux's 9p net/9p, include/net/9p, and various Go 9p
// implementations, as well as the this 9P paper:
// https://www.usenix.org/legacy/events/usenix05/tech/freenix/hensbergen.html
package ninep

import (
	"fmt"
	"strconv"

	"sigmaos/sessp"
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

type Tseqno uint64

// NoSeqno signifies the fcall came from a wire-compatible peer
const NoSeqno Tseqno = ^Tseqno(0)

// NoTag is the tag for Tversion and Rversion requests.
const NoTag Ttag = ^Ttag(0)

// NoFid is a reserved fid used in a Tattach request for the afid
// field, that indicates that the client does not wish to authenticate
// this session.
const NoFid Tfid = ^Tfid(0)
const NoOffset Toffset = ^Toffset(0)

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

type Qtype9P uint8
type TQversion uint32

const NoPath Tpath = ^Tpath(0)
const NoV TQversion = ^TQversion(0)

func VEq(v1, v2 TQversion) bool {
	return v1 == NoV || v1 == v2
}

// A Qid's type field represents the type of a file, the high 8 bits of
// the file's permission.
const (
	QTDIR     Qtype9P = 0x80 // directories
	QTAPPEND  Qtype9P = 0x40 // append only files
	QTEXCL    Qtype9P = 0x20 // exclusive use files
	QTMOUNT   Qtype9P = 0x10 // mounted channel
	QTAUTH    Qtype9P = 0x08 // authentication file (afid)
	QTTMP     Qtype9P = 0x04 // non-backed-up file
	QTSYMLINK Qtype9P = 0x02
	QTFILE    Qtype9P = 0x00
)

func (qt Qtype9P) String() string {
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
type Tqid9P struct {
	Type    Qtype9P
	Version TQversion
	Path    Tpath
}

func NewQid(t Qtype9P, v TQversion, p Tpath) Tqid9P {
	return Tqid9P{t, v, p}
}

func NewQidPerm(perm Tperm, v TQversion, p Tpath) Tqid9P {
	return NewQid(Qtype9P(perm>>QTYPESHIFT), v, p)
}

func (q Tqid9P) String() string {
	return fmt.Sprintf("{%v v %v p %v}", q.Type, q.Version, q.Path)
}

type Tmode9P uint8

// Flags for the mode field in Topen and Tcreate messages
const (
	OREAD   Tmode9P = 0    // read-only
	OWRITE  Tmode9P = 0x01 // write-only
	ORDWR   Tmode9P = 0x02 // read-write
	OEXEC   Tmode9P = 0x03 // execute (implies OREAD)
	OTRUNC  Tmode9P = 0x10 // or truncate file first
	OCEXEC  Tmode9P = 0x20 // or close on exec
	ORCLOSE Tmode9P = 0x40 // remove on close
	OAPPEND Tmode9P = 0x80 // append
)

func (m Tmode9P) String() string {
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
func (p Tperm) IsFile() bool       { return (p>>QTYPESHIFT)&0xFF == 0 }

func (p Tperm) String() string {
	qt := Qtype9P(p >> QTYPESHIFT)
	return fmt.Sprintf("qt %v qp %x", qt, uint8(p&TYPEMASK))
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
	Aqid Tqid9P
}

type Tattach9P struct {
	Fid   Tfid
	Afid  Tfid
	Uname string
	Aname string
}

func (m Tattach9P) String() string {
	return fmt.Sprintf("{%v a %v u %v a '%v'}", m.Fid, m.Afid, m.Uname, m.Aname)
}

type Rattach9P struct {
	Qid Tqid9P
}

type Rerror9P struct {
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
	Qids []Tqid9P
}

type Topen9P struct {
	Fid  Tfid
	Mode Tmode9P
}

type Twatch struct {
	Fid Tfid
}

type Ropen struct {
	Qid    Tqid9P
	Iounit Tiounit
}

type Tcreate9P struct {
	Fid  Tfid
	Name string
	Perm Tperm
	Mode Tmode9P
}

type Rcreate struct {
	Qid    Tqid9P
	Iounit Tiounit
}

type Tread struct {
	Fid    Tfid
	Offset Toffset
	Count  Tsize
}

type Rread9P struct {
	Data []byte
}

func (rr Rread9P) String() string {
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

type Rwrite struct {
	Count Tsize
}

type Tclunk struct {
	Fid Tfid
}

type Rclunk struct {
}

type Tremove9P struct {
	Fid Tfid
}

type Rremove struct {
}

type Tstat struct {
	Fid Tfid
}

type Stat9P struct {
	Type   uint16
	Dev    uint32
	Qid    Tqid9P
	Mode   Tperm
	Atime  uint32  // last access time in seconds
	Mtime  uint32  // last modified time in seconds
	Length Tlength // file length in bytes
	Name   string  // file name
	Uid    string  // owner name
	Gid    string  // group name
	Muid   string  // name of the last user that modified the file
}

func (s Stat9P) String() string {
	return fmt.Sprintf("stat(%v mode=%v atime=%v mtime=%v length=%v name=%v uid=%v gid=%v muid=%v)",
		s.Qid, s.Mode, s.Atime, s.Mtime, s.Length, s.Name, s.Uid, s.Gid, s.Muid)
}

type Rstat9P struct {
	Size uint16 // extra Size, see stat(5)
	Stat Stat9P
}

type Twstat9P struct {
	Fid  Tfid
	Size uint16 // extra Size, see stat(5)
	Stat Stat9P
}

type Rwstat struct{}

func (Rerror9P) Type() sessp.Tfcall  { return sessp.TRerror9P }
func (Tattach9P) Type() sessp.Tfcall { return sessp.TTattach }
func (Tflush) Type() sessp.Tfcall    { return sessp.TTflush }
func (Rflush) Type() sessp.Tfcall    { return sessp.TRflush }
func (Tcreate9P) Type() sessp.Tfcall { return sessp.TTcreate }
func (Topen9P) Type() sessp.Tfcall   { return sessp.TTopen }
func (Tread) Type() sessp.Tfcall     { return sessp.TTread }
func (Rread9P) Type() sessp.Tfcall   { return sessp.TRread9P }
func (Twrite) Type() sessp.Tfcall    { return sessp.TTwrite }
func (Rstat9P) Type() sessp.Tfcall   { return sessp.TRstat9P }
func (Tremove9P) Type() sessp.Tfcall { return sessp.TTremove }
func (Twstat9P) Type() sessp.Tfcall  { return sessp.TTwstat }
