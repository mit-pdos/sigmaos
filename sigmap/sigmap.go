package sigmap

//
// Go structures for sigmap protocol, which is based on 9P.
//

import (
	"fmt"

	"sigmaos/path"
	"sigmaos/serr"
	"sigmaos/sessp"
)

type Tfid uint32
type Tiounit uint32
type Tperm uint32
type Toffset uint64
type Tlength uint64
type Tgid uint32
type Trealm string

const ROOTREALM Trealm = "rootrealm"

func (r Trealm) String() string {
	return string(r)
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
const NoOffset Toffset = ^Toffset(0)

// If need more than MaxGetSet, use Open/Read/Close interface
const MAXGETSET sessp.Tsize = 1_000_000

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

func MakeQid(t Qtype, v TQversion, p sessp.Tpath) *Tqid {
	return &Tqid{Type: uint32(t), Version: uint32(v), Path: uint64(p)}
}

func MakeQidPerm(perm Tperm, v TQversion, p sessp.Tpath) *Tqid {
	return MakeQid(Qtype(perm>>QTYPESHIFT), v, p)
}

func (qid *Tqid) Tversion() TQversion {
	return TQversion(qid.Version)
}

func (qid *Tqid) Tpath() sessp.Tpath {
	return sessp.Tpath(qid.Path)
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

func MkErr(msg *Rerror) *serr.Err {
	return &serr.Err{serr.Terror(msg.ErrCode), msg.Obj, fmt.Errorf("%s", msg.Err)}
}

func MkRerror(err *serr.Err) *Rerror {
	return &Rerror{ErrCode: uint32(err.ErrCode), Obj: err.Obj, Err: err.String()}
}

func MkRerrorCode(ec serr.Terror) *Rerror {
	return &Rerror{ErrCode: uint32(ec)}
}

func MkTwalk(fid, nfid Tfid, p path.Path) *Twalk {
	return &Twalk{Fid: uint32(fid), NewFid: uint32(nfid), Wnames: p}
}

func (w *Twalk) Tfid() Tfid {
	return Tfid(w.Fid)
}

func (w *Twalk) Tnewfid() Tfid {
	return Tfid(w.NewFid)
}

func MkTattach(fid, afid Tfid, uname string, path path.Path) *Tattach {
	return &Tattach{Fid: uint32(fid), Afid: uint32(afid), Uname: uname, Aname: path.String()}
}

func (a *Tattach) Tfid() Tfid {
	return Tfid(a.Fid)
}

func MkTopen(fid Tfid, mode Tmode) *Topen {
	return &Topen{Fid: uint32(fid), Mode: uint32(mode)}
}

func (o *Topen) Tfid() Tfid {
	return Tfid(o.Fid)
}

func (o *Topen) Tmode() Tmode {
	return Tmode(o.Mode)
}

func MkTcreate(fid Tfid, n string, p Tperm, mode Tmode) *Tcreate {
	return &Tcreate{Fid: uint32(fid), Name: n, Perm: uint32(p), Mode: uint32(mode)}
}

func (c *Tcreate) Tfid() Tfid {
	return Tfid(c.Fid)
}

func (c *Tcreate) Tperm() Tperm {
	return Tperm(c.Perm)
}

func (c *Tcreate) Tmode() Tmode {
	return Tmode(c.Mode)
}

func MkReadV(fid Tfid, o Toffset, c sessp.Tsize, v TQversion) *TreadV {
	return &TreadV{Fid: uint32(fid), Offset: uint64(o), Count: uint32(c), Version: uint32(v)}
}

func (r *TreadV) Tfid() Tfid {
	return Tfid(r.Fid)
}

func (r *TreadV) Tversion() TQversion {
	return TQversion(r.Version)
}

func (r *TreadV) Toffset() Toffset {
	return Toffset(r.Offset)
}

func (r *TreadV) Tcount() sessp.Tsize {
	return sessp.Tsize(r.Count)
}

func MkTwriteV(fid Tfid, o Toffset, v TQversion) *TwriteV {
	return &TwriteV{Fid: uint32(fid), Offset: uint64(o), Version: uint32(v)}
}

func (w *TwriteV) Tfid() Tfid {
	return Tfid(w.Fid)
}

func (w *TwriteV) Toffset() Toffset {
	return Toffset(w.Offset)
}

func (w *TwriteV) Tversion() TQversion {
	return TQversion(w.Version)
}

func (wr *Rwrite) Tcount() sessp.Tsize {
	return sessp.Tsize(wr.Count)
}

func MkTwatch(fid Tfid) *Twatch {
	return &Twatch{Fid: uint32(fid)}
}

func (w *Twatch) Tfid() Tfid {
	return Tfid(w.Fid)
}

func MkTclunk(fid Tfid) *Tclunk {
	return &Tclunk{Fid: uint32(fid)}
}

func (c *Tclunk) Tfid() Tfid {
	return Tfid(c.Fid)
}

func MkTremove(fid Tfid) *Tremove {
	return &Tremove{Fid: uint32(fid)}
}

func (r *Tremove) Tfid() Tfid {
	return Tfid(r.Fid)
}

func MkTstat(fid Tfid) *Tstat {
	return &Tstat{Fid: uint32(fid)}
}

func (s *Tstat) Tfid() Tfid {
	return Tfid(s.Fid)
}

func MkStatNull() *Stat {
	st := &Stat{}
	st.Qid = MakeQid(0, 0, 0)
	return st
}

func MkStat(qid *Tqid, perm Tperm, mtime uint32, name, owner string) *Stat {
	st := &Stat{
		Type:   0, // XXX
		Qid:    qid,
		Mode:   uint32(perm),
		Atime:  0,
		Mtime:  mtime,
		Name:   name,
		Length: 0,
		Uid:    owner,
		Gid:    owner,
		Muid:   "",
	}
	return st

}

func (st *Stat) Tlength() Tlength {
	return Tlength(st.Length)
}

func (st *Stat) Tmode() Tperm {
	return Tperm(st.Mode)
}

func Names(sts []*Stat) []string {
	r := []string{}
	for _, st := range sts {
		r = append(r, st.Name)
	}
	return r
}

func MkTwstat(fid Tfid, st *Stat) *Twstat {
	return &Twstat{Fid: uint32(fid), Stat: st}
}

func (w *Twstat) Tfid() Tfid {
	return Tfid(w.Fid)
}

func MkTrenameat(oldfid Tfid, oldname string, newfid Tfid, newname string) *Trenameat {
	return &Trenameat{OldFid: uint32(oldfid), OldName: oldname, NewFid: uint32(newfid), NewName: newname}
}

func (r *Trenameat) Tnewfid() Tfid {
	return Tfid(r.NewFid)
}

func (r *Trenameat) Toldfid() Tfid {
	return Tfid(r.OldFid)
}

func MkTgetfile(fid Tfid, mode Tmode, offset Toffset, cnt sessp.Tsize, path path.Path, resolve bool) *Tgetfile {
	return &Tgetfile{Fid: uint32(fid), Mode: uint32(mode), Offset: uint64(offset), Count: uint32(cnt), Wnames: path, Resolve: resolve}
}

func (g *Tgetfile) Tfid() Tfid {
	return Tfid(g.Fid)
}

func (g *Tgetfile) Tmode() Tmode {
	return Tmode(g.Mode)
}

func (g *Tgetfile) Toffset() Toffset {
	return Toffset(g.Offset)
}

func (g *Tgetfile) Tcount() sessp.Tsize {
	return sessp.Tsize(g.Count)
}

func MkTsetfile(fid Tfid, mode Tmode, offset Toffset, path path.Path, resolve bool) *Tsetfile {
	return &Tsetfile{Fid: uint32(fid), Mode: uint32(mode), Offset: uint64(offset), Wnames: path, Resolve: resolve}
}

func (s *Tsetfile) Tfid() Tfid {
	return Tfid(s.Fid)
}

func (s *Tsetfile) Tmode() Tmode {
	return Tmode(s.Mode)
}

func (s *Tsetfile) Toffset() Toffset {
	return Toffset(s.Offset)
}

func MkTputfile(fid Tfid, mode Tmode, perm Tperm, offset Toffset, path path.Path) *Tputfile {
	return &Tputfile{Fid: uint32(fid), Mode: uint32(mode), Perm: uint32(perm), Offset: uint64(offset), Wnames: path}
}

func (p *Tputfile) Tfid() Tfid {
	return Tfid(p.Fid)
}

func (p *Tputfile) Tmode() Tmode {
	return Tmode(p.Mode)
}

func (p *Tputfile) Tperm() Tperm {
	return Tperm(p.Perm)
}

func (p *Tputfile) Toffset() Toffset {
	return Toffset(p.Offset)
}

func MkTremovefile(fid Tfid, path path.Path, r bool) *Tremovefile {
	return &Tremovefile{Fid: uint32(fid), Wnames: path, Resolve: r}
}

func (r *Tremovefile) Tfid() Tfid {
	return Tfid(r.Fid)
}

func MkTheartbeat(sess []uint64) *Theartbeat {
	return &Theartbeat{Sids: sess}
}

func MkTdetach(pid, lid uint32) *Tdetach {
	return &Tdetach{PropId: pid, LeadId: lid}
}

func MkTwriteread(fid Tfid) *Twriteread {
	return &Twriteread{Fid: uint32(fid)}
}

func (wr *Twriteread) Tfid() Tfid {
	return Tfid(wr.Fid)
}

func (Tversion) Type() sessp.Tfcall { return sessp.TTversion }
func (Rversion) Type() sessp.Tfcall { return sessp.TRversion }
func (Tauth) Type() sessp.Tfcall    { return sessp.TTauth }
func (Rauth) Type() sessp.Tfcall    { return sessp.TRauth }
func (Tattach) Type() sessp.Tfcall  { return sessp.TTattach }
func (Rattach) Type() sessp.Tfcall  { return sessp.TRattach }
func (Rerror) Type() sessp.Tfcall   { return sessp.TRerror }
func (Twalk) Type() sessp.Tfcall    { return sessp.TTwalk }
func (Rwalk) Type() sessp.Tfcall    { return sessp.TRwalk }
func (Topen) Type() sessp.Tfcall    { return sessp.TTopen }
func (Twatch) Type() sessp.Tfcall   { return sessp.TTwatch }
func (Ropen) Type() sessp.Tfcall    { return sessp.TRopen }
func (Tcreate) Type() sessp.Tfcall  { return sessp.TTcreate }
func (Rcreate) Type() sessp.Tfcall  { return sessp.TRcreate }
func (Rread) Type() sessp.Tfcall    { return sessp.TRread }
func (Rwrite) Type() sessp.Tfcall   { return sessp.TRwrite }
func (Tclunk) Type() sessp.Tfcall   { return sessp.TTclunk }
func (Rclunk) Type() sessp.Tfcall   { return sessp.TRclunk }
func (Tremove) Type() sessp.Tfcall  { return sessp.TTremove }
func (Rremove) Type() sessp.Tfcall  { return sessp.TRremove }
func (Tstat) Type() sessp.Tfcall    { return sessp.TTstat }
func (Twstat) Type() sessp.Tfcall   { return sessp.TTwstat }
func (Rwstat) Type() sessp.Tfcall   { return sessp.TRwstat }

//
// sigmaP
//

func (Rstat) Type() sessp.Tfcall       { return sessp.TRstat }
func (TreadV) Type() sessp.Tfcall      { return sessp.TTreadV }
func (TwriteV) Type() sessp.Tfcall     { return sessp.TTwriteV }
func (Trenameat) Type() sessp.Tfcall   { return sessp.TTrenameat }
func (Rrenameat) Type() sessp.Tfcall   { return sessp.TRrenameat }
func (Tremovefile) Type() sessp.Tfcall { return sessp.TTremovefile }
func (Tgetfile) Type() sessp.Tfcall    { return sessp.TTgetfile }
func (Tsetfile) Type() sessp.Tfcall    { return sessp.TTsetfile }
func (Tputfile) Type() sessp.Tfcall    { return sessp.TTputfile }
func (Tdetach) Type() sessp.Tfcall     { return sessp.TTdetach }
func (Rdetach) Type() sessp.Tfcall     { return sessp.TRdetach }
func (Theartbeat) Type() sessp.Tfcall  { return sessp.TTheartbeat }
func (Rheartbeat) Type() sessp.Tfcall  { return sessp.TRheartbeat }
func (Twriteread) Type() sessp.Tfcall  { return sessp.TTwriteread }
