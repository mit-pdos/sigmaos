package sigmap

//
// Go structures for sigmap protocol, which is based on 9P.
//

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"sigmaos/rand"
)

type Tfid uint32
type Tpath uint64
type Tiounit uint32
type Tperm uint32
type Toffset uint64
type Tsize uint32
type Tlength uint64
type Tgid uint32
type Trealm string
type Tpid string
type Tuname string
type TclntId uint64
type TleaseId uint64
type Tttl uint64
type Thost string
type Tport uint32

const ROOTREALM Trealm = "rootrealm"

func GenPid(program string) Tpid {
	return Tpid(program + "-" + rand.String(8))
}

func (r Trealm) String() string {
	return string(r)
}

func (pid Tpid) String() string {
	return string(pid)
}

func (fid Tfid) String() string {
	if fid == NoFid {
		return "-1"
	}
	return fmt.Sprintf("fid %d", fid)
}

// NoFid is a reserved fid used in a Tattach request for the afid
// field, that indicates that the client does not wish to authenticate
// this session.
const NoFid Tfid = ^Tfid(0)

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
const NoOffset Toffset = ^Toffset(0)
const NoClntId TclntId = ^TclntId(0)
const NoLeaseId TleaseId = ^TleaseId(0)

func (lid TleaseId) String() string {
	return strconv.FormatUint(uint64(lid), 16)
}

func (cid TclntId) String() string {
	return strconv.FormatUint(uint64(cid), 16)
}

// If need more than MaxGetSet, use Open/Read/Close interface
const MAXGETSET Tsize = 1_000_000

type Qtype uint32
type TQversion uint32

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

func NewQid(t Qtype, v TQversion, p Tpath) *Tqid {
	return &Tqid{Type: uint32(t), Version: uint32(v), Path: uint64(p)}
}

func NewQidPerm(perm Tperm, v TQversion, p Tpath) *Tqid {
	return NewQid(Qtype(perm>>QTYPESHIFT), v, p)
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

type Tmode uint32

// Flags for the mode field in Topen and Tcreate messages
const (
	OREAD   Tmode = 0    // read-only
	OWRITE  Tmode = 0x01 // write-only
	ORDWR   Tmode = 0x02 // read-write
	OEXEC   Tmode = 0x03 // execute (implies OREAD)
	OEXCL   Tmode = 0x04 // exclusive
	OTRUNC  Tmode = 0x10 // or truncate file first
	OCEXEC  Tmode = 0x20 // or close on exec
	ORCLOSE Tmode = 0x40 // remove on close
	OAPPEND Tmode = 0x80 // append
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

func (p Tport) String() string {
	return strconv.FormatUint(uint64(p), 10)
}

func (p Thost) String() string {
	return string(p)
}

func ParsePort(ps string) (Tport, error) {
	pi, err := strconv.ParseUint(ps, 10, 32)
	return Tport(pi), err
}

const (
	NO_HOST   Thost = ""
	LOCALHOST Thost = "127.0.0.1"
	NO_PORT   Tport = 0
)

const ()

func (a *Taddr) HostPort() string {
	return a.HostStr + ":" + a.GetPort().String()
}

func (a *Taddr) GetHost() Thost {
	return Thost(a.HostStr)
}

func (a *Taddr) GetPort() Tport {
	return Tport(a.PortInt)
}

func NewTaddrAnyPort(netns string) *Taddr {
	return NewTaddrRealm(NO_HOST, NO_PORT, netns)
}

func NewTaddr(host Thost, port Tport) *Taddr {
	return &Taddr{
		HostStr: string(host),
		PortInt: uint32(port),
		NetNS:   ROOTREALM.String(),
	}
}

func NewTaddrRealm(host Thost, port Tport, netns string) *Taddr {
	return &Taddr{
		HostStr: string(host),
		PortInt: uint32(port),
		NetNS:   netns,
	}
}

func (a *Taddr) Marshal() string {
	b, err := json.Marshal(a)
	if err != nil {
		log.Fatalf("Can't marshal Taddr: %v", err)
	}
	return string(b)
}

func UnmarshalTaddr(a string) *Taddr {
	var addr Taddr
	err := json.Unmarshal([]byte(a), &addr)
	if err != nil {
		log.Fatalf("Can't unmarshal Taddr")
	}
	return &addr
}

type Taddrs []*Taddr

//func NewTaddrs(addr []string) Taddrs {
//	addrs := make([]*Taddr, len(addr))
//	for i, a := range addr {
//		addrs[i] = NewTaddr(a)
//	}
//	return addrs
//}

// Ignores net
func (as Taddrs) String() string {
	s := ""
	for i, a := range as {
		s += a.HostPort()
		if i < len(as)-1 {
			s += ","
		}
	}
	return s
}

// Includes net. In the future should return a mnt, but we need to
// package it up in a way suitable to pass as argument or environment
// variable to a program.
func (as Taddrs) Taddrs2String() (string, error) {
	b, err := json.Marshal(as)
	return string(b), err
}

func String2Taddrs(as string) (Taddrs, error) {
	var addrs Taddrs
	err := json.Unmarshal([]byte(as), &addrs)
	return addrs, err
}
