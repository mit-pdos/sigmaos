package ninep

// Go structures for 9P based on the wire format in fcall.h and styx

type Tsize uint32
type Ttag uint16
type Tfid uint32
type Tiounit uint32
type Tperm uint32
type Toffset uint64

// NoTag is the tag for Tversion and Rversion requests.
const NoTag Ttag = 0

// NoFid is a reserved fid used in a Tattach request for the afid
// field, that indicates that the client does not wish to authenticate
// this session.
const NoFid Tfid = 0

type Tpath uint64
type Qtype uint8
type TQversion uint32

// A Qid's type field represents the type of a file (directory, etc.),
// represented as a bit vector corresponding to the high 8 bits of the
// file's mode word.
const (
	QTDIR    Qtype = 0x80 // directories
	QTAPPEND Qtype = 0x40 // append only files
	QTEXCL   Qtype = 0x20 // exclusive use files
	QTMOUNT  Qtype = 0x10 // mounted channel
	QTAUTH   Qtype = 0x08 // authentication file (afid)
	QTTMP    Qtype = 0x04 // non-backed-up file
	QTFILE   Qtype = 0x00
)

// A Qid is the server's unique identification for the file being
// accessed: two files on the same server hierarchy are the same if
// and only if their qids are the same.
type Tqid struct {
	Type    Qtype
	Version TQversion
	Path    Tpath
}

func MakeQid(t Qtype, v TQversion, p Tpath) *Tqid {
	return &Tqid{t, v, p}
}

type Tmode uint16

// Flags for the mode field in Topen and Tcreate messages
const (
	OREAD   Tmode = 0  // open read-only
	OWRITE  Tmode = 1  // open write-only
	ORDWR   Tmode = 2  // open read-write
	OEXEC   Tmode = 3  // execute (== read but check execute permission)
	OTRUNC  Tmode = 16 // or'ed in (except for exec), truncate file first
	OCEXEC  Tmode = 32 // or'ed in, close on exec
	ORCLOSE Tmode = 64 // or'ed in, remove on close
)

// File modes
const (
	DMDIR    Tperm = 0x80000000 // mode bit for directories
	DMAPPEND Tperm = 0x40000000 // mode bit for append only files
	DMEXCL   Tperm = 0x20000000 // mode bit for exclusive use files
	DMMOUNT  Tperm = 0x10000000 // mode bit for mounted channel
	DMAUTH   Tperm = 0x08000000 // mode bit for authentication file
	DMTMP    Tperm = 0x04000000 // mode bit for non-backed-up file
	DMREAD   Tperm = 0x4        // mode bit for read permission
	DMWRITE  Tperm = 0x2        // mode bit for write permission
	DMEXEC   Tperm = 0x1        // mode bit for execute permission

	// Mask for the type bits
	DMTYPE = DMDIR | DMAPPEND | DMEXCL | DMMOUNT | DMTMP

	// Mask for the permissions bits
	DMPERM = DMREAD | DMWRITE | DMEXEC
)

type Tversion struct {
	Tag     Ttag
	Msize   Tsize
	Version []string
}

type Rversion struct {
	Tag     Ttag
	Msize   Tsize
	Version []string
}

type Tauth struct {
	Tag    Ttag
	Afid   Tfid
	Unames []string
	Anames []string
}

type Rauth struct {
	Tag  Ttag
	Aqid Tqid
}

type Tattach struct {
	Tag   Ttag
	Fid   Tfid
	Afid  Tfid
	Uname string
	Aname string
}

type Rattach struct {
	Tag Ttag
	Qid Tqid
}

type Tflush struct {
	Tag    Ttag
	Oldtag Ttag
}

type Rflush struct {
	Tag Ttag
}

type Twalk struct {
	Tag    Ttag
	Fid    Tfid
	NewFid Tfid
	Path   []string
}

type Rwalk struct {
	Tag  Ttag
	Qids []Tqid
}

type Topen struct {
	Tag  Ttag
	Fid  Tfid
	Mode Tmode
}

type Ropen struct {
	Tag    Ttag
	Qid    Tqid
	Iounit Tiounit
}

type Tcreate struct {
	Tag  Ttag
	Fid  Tfid
	Name string
	Perm Tperm
	Mode Tmode
}

type Rcreate struct {
	Tag    Ttag
	Qid    Tqid
	Iounit Tiounit
}

type Tread struct {
	Tag    Ttag
	Fid    Tfid
	Offset Toffset
	Count  Tsize
}

type Rread struct {
	Tag  Ttag
	Data []byte
}

type Twrite struct {
	Tag    Ttag
	Fid    Tfid
	Offset Toffset
	Data   []byte
}

type Rwrite struct {
	Tag   Ttag
	Count Tsize
}

type Tclunk struct {
	Tag Ttag
	Fid Tfid
}

type Rclunk struct {
	Tag Ttag
}

type Tremove struct {
	Tag Ttag
	Fid Tfid
}

type Rremove struct {
	Tag Ttag
}

type Trstat struct {
	Tag Ttag
	Fid Tfid
}

type Rrstat struct {
	Tag  Ttag
	Stat []byte
}

type Twstat struct {
	Tag  Ttag
	Fid  Tfid
	Stat []byte
}

type Rwstat struct {
	Tag Ttag
}
