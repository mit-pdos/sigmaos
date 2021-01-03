package ninep

//
// Go structures for 9P based on the wire format in Linux's 9p net/9p,
// include/net/9p, styx, and
// https://github.com/chaos/diod/blob/master/protocol.md
//

type Tsize uint32
type Ttag uint16
type Tfid uint32
type Tiounit uint32
type Tperm uint32
type Toffset uint64
type Tgid uint32

// NoTag is the tag for Tversion and Rversion requests.
const NoTag Ttag = 0

// NoFid is a reserved fid used in a Tattach request for the afid
// field, that indicates that the client does not wish to authenticate
// this session.
const NoFid Tfid = 0

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

type Tmode uint16

// Flags for the mode field in Topen and Tcreate messages
const (
	OREAD   Tmode = 0    // read-only
	OWRITE  Tmode = 0x01 // write-only
	ORDWR   Tmode = 0x02 // read-write
	OEXEC   Tmode = 0x03 // execute (implies OREAD)
	OTRUNC  Tmode = 0x10 // truncate file first
	OREXEC  Tmode = 0x20 // close on exec
	ORCLOSE Tmode = 0x40 // remove on close
	OAPPEND Tmode = 0x80 // append
	OEXCL   Tmode = 0x1000
)

// Permissions
const (
	DMDIR    Tperm = 0x80000000 // directory
	DMAPPEND Tperm = 0x40000000 // append only file
	DMEXCL   Tperm = 0x20000000 // exclusive use file
	DMMOUNT  Tperm = 0x10000000 // mounted channel
	DMAUTH   Tperm = 0x08000000 // authentication file
	DMTMP    Tperm = 0x04000000 // non-backed-up file

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

type Dirent struct {
	Qid    Tqid
	Offset Toffset
	Type   Tperm
	Name   string
}

type Tmkdir struct {
	Tag  Ttag
	Dfid Tfid
	Name string
	Mode Tmode
	Gid  Tgid
}

type Rmkdir struct {
	Tag Ttag
	Qid Tqid
}

type Treaddir struct {
	Tag    Ttag
	Fid    Tfid
	Offset Toffset
	Count  Tsize
}

type Rreaddir struct {
	Tag  Ttag
	Data []byte
}

type Tsymlink struct {
	Tag    Ttag
	Fid    Tfid
	Name   string
	Symtgt string
	Gid    Tgid
}

type Rsymlink struct {
	Tag Ttag
	Qid Tqid
}

type Treadlink struct {
	Tag Ttag
	Fid Tfid
}

type Rreadlink struct {
	Tag    Ttag
	Target string
}

//
// New transactions
//

type Tpipe struct {
	Tag  Ttag
	Dfid Tfid
	Name string
	Mode Tmode
	Gid  Tgid
}

type Rpipe struct {
	Tag Ttag
	Qid Tqid
}
