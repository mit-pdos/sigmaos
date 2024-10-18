package sigmap

import (
	"fmt"

	"sigmaos/path"
	"sigmaos/serr"
	"sigmaos/sessp"
)

func (r *Rerror) TErrCode() serr.Terror {
	return serr.Terror(r.ErrCode)
}

func NewErr(msg *Rerror) *serr.Err {
	return &serr.Err{serr.Terror(msg.ErrCode), msg.Obj, fmt.Errorf("%s", msg.Err)}
}

func NewRerrorSerr(err *serr.Err) *Rerror {
	r := ""
	if err.Err != nil {
		r = err.Err.Error()
	}
	return &Rerror{ErrCode: uint32(err.ErrCode), Obj: err.Obj, Err: r}
}

func NewRerrorErr(err error) *Rerror {
	return &Rerror{ErrCode: uint32(serr.TErrError), Obj: err.Error()}
}

func NewRerror() *Rerror {
	return &Rerror{ErrCode: 0}
}

func NewRerrorCode(ec serr.Terror) *Rerror {
	return &Rerror{ErrCode: uint32(ec)}
}

func NewTwalk(fid, nfid Tfid, p path.Tpathname) *Twalk {
	return &Twalk{Fid: uint32(fid), NewFid: uint32(nfid), Wnames: p}
}

func (w *Twalk) Tfid() Tfid {
	return Tfid(w.Fid)
}

func (w *Twalk) Tnewfid() Tfid {
	return Tfid(w.NewFid)
}

func NewTattach(fid, afid Tfid, secrets map[string]*SecretProto, cid TclntId, path path.Tpathname) *Tattach {
	return &Tattach{Fid: uint32(fid), Afid: uint32(afid), Secrets: secrets, Aname: path.String(), ClntId: uint64(cid)}
}

func (a *Tattach) Tfid() Tfid {
	return Tfid(a.Fid)
}

func (a *Tattach) Tafid() Tfid {
	return Tfid(a.Afid)
}

func (a *Tattach) TclntId() TclntId {
	return TclntId(a.ClntId)
}

func NewTopen(fid Tfid, mode Tmode) *Topen {
	return &Topen{Fid: uint32(fid), Mode: uint32(mode)}
}

func (o *Topen) Tfid() Tfid {
	return Tfid(o.Fid)
}

func (o *Topen) Tmode() Tmode {
	return Tmode(o.Mode)
}

func NewTcreate(fid Tfid, n string, p Tperm, mode Tmode, lid TleaseId, f Tfence) *Tcreate {
	return &Tcreate{Fid: uint32(fid), Name: n, Perm: uint32(p), Mode: uint32(mode), Lease: uint64(lid), Fence: f.FenceProto()}
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

func (c *Tcreate) TleaseId() TleaseId {
	return TleaseId(c.Lease)
}

func (c *Tcreate) Tfence() Tfence {
	return c.Fence.Tfence()
}

func NewReadF(fid Tfid, o Toffset, c Tsize, f *Tfence) *TreadF {
	return &TreadF{Fid: uint32(fid), Offset: uint64(o), Count: uint32(c), Fence: f.FenceProto()}
}

func (r *TreadF) Tfid() Tfid {
	return Tfid(r.Fid)
}

func (r *TreadF) Toffset() Toffset {
	return Toffset(r.Offset)
}

func (r *TreadF) Tcount() Tsize {
	return Tsize(r.Count)
}

func (r *TreadF) Tfence() Tfence {
	return r.Fence.Tfence()
}

func (r *Rread) Tcount() Tsize {
	return Tsize(r.Count)
}

func NewTwriteF(fid Tfid, o Toffset, f *Tfence) *TwriteF {
	return &TwriteF{Fid: uint32(fid), Offset: uint64(o), Fence: f.FenceProto()}
}

func (w *TwriteF) Tfid() Tfid {
	return Tfid(w.Fid)
}

func (w *TwriteF) Toffset() Toffset {
	return Toffset(w.Offset)
}

func (w *TwriteF) Tfence() Tfence {
	return w.Fence.Tfence()
}

func (wr *Rwrite) Tcount() Tsize {
	return Tsize(wr.Count)
}

func NewTwatch(fid Tfid) *Twatch {
	return &Twatch{Fid: uint32(fid)}
}

func (w *Twatch) Tfid() Tfid {
	return Tfid(w.Fid)
}

func NewTwatchv2(dirfid Tfid, watchfid Tfid) *Twatchv2 {
	return &Twatchv2{Dirfid: uint32(dirfid), Watchfid: uint32(watchfid)}
}

func (w *Twatchv2) Tdirfid() Tfid {
	return Tfid(w.Dirfid)
}

func (w *Twatchv2) Twatchfid() Tfid {
	return Tfid(w.Watchfid)
}

func NewTclunk(fid Tfid) *Tclunk {
	return &Tclunk{Fid: uint32(fid)}
}

func (c *Tclunk) Tfid() Tfid {
	return Tfid(c.Fid)
}

func NewTremove(fid Tfid, f *Tfence) *Tremove {
	return &Tremove{Fid: uint32(fid), Fence: f.FenceProto()}
}

func (r *Tremove) Tfid() Tfid {
	return Tfid(r.Fid)
}

func (r *Tremove) Tfence() Tfence {
	return r.Fence.Tfence()
}

func NewTrstat(fid Tfid) *Trstat {
	return &Trstat{Fid: uint32(fid)}
}

func (s *Trstat) Tfid() Tfid {
	return Tfid(s.Fid)
}

func NewTwstat(fid Tfid, st *Tstat, f *Tfence) *Twstat {
	return &Twstat{Fid: uint32(fid), Stat: st.StatProto(), Fence: f.FenceProto()}
}

func (w *Twstat) Tfid() Tfid {
	return Tfid(w.Fid)
}

func (w *Twstat) Tfence() Tfence {
	return w.Fence.Tfence()
}

func NewTrenameat(oldfid Tfid, oldname string, newfid Tfid, newname string, f *Tfence) *Trenameat {
	return &Trenameat{OldFid: uint32(oldfid), OldName: oldname, NewFid: uint32(newfid), NewName: newname, Fence: f.FenceProto()}
}

func (r *Trenameat) Tnewfid() Tfid {
	return Tfid(r.NewFid)
}

func (r *Trenameat) Toldfid() Tfid {
	return Tfid(r.OldFid)
}

func (r *Trenameat) Tfence() Tfence {
	return r.Fence.Tfence()
}

func NewTgetfile(fid Tfid, mode Tmode, offset Toffset, cnt Tsize, path path.Tpathname, resolve bool, f *Tfence) *Tgetfile {
	return &Tgetfile{Fid: uint32(fid), Mode: uint32(mode), Offset: uint64(offset), Count: uint32(cnt), Wnames: path, Resolve: resolve, Fence: f.FenceProto()}
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

func (g *Tgetfile) Tcount() Tsize {
	return Tsize(g.Count)
}

func (g *Tgetfile) Tfence() Tfence {
	return g.Fence.Tfence()
}

func NewTputfile(fid Tfid, mode Tmode, perm Tperm, offset Toffset, path path.Tpathname, resolve bool, lid TleaseId, f *Tfence) *Tputfile {
	return &Tputfile{Fid: uint32(fid), Mode: uint32(mode), Perm: uint32(perm), Offset: uint64(offset), Wnames: path, Resolve: resolve, Lease: uint64(lid), Fence: f.FenceProto()}
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

func (p *Tputfile) TleaseId() TleaseId {
	return TleaseId(p.Lease)
}

func (p *Tputfile) Tfence() Tfence {
	return p.Fence.Tfence()
}

func NewTremovefile(fid Tfid, path path.Tpathname, r bool, f *Tfence) *Tremovefile {
	return &Tremovefile{Fid: uint32(fid), Wnames: path, Resolve: r, Fence: f.FenceProto()}
}

func (r *Tremovefile) Tfid() Tfid {
	return Tfid(r.Fid)
}

func (r *Tremovefile) Tfence() Tfence {
	return r.Fence.Tfence()
}

func NewTheartbeat(sess map[uint64]bool) *Theartbeat {
	return &Theartbeat{Sids: sess}
}

func NewTdetach(cid TclntId) *Tdetach {
	return &Tdetach{ClntId: uint64(cid)}
}

func (d *Tdetach) TclntId() TclntId {
	return TclntId(d.ClntId)
}

func NewTwriteread(fid Tfid) *Twriteread {
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
func (Ropen) Type() sessp.Tfcall    { return sessp.TRopen }
func (Twatch) Type() sessp.Tfcall   { return sessp.TTwatch }
func (Twatchv2) Type() sessp.Tfcall { return sessp.TTwatchv2 }
func (Rwatchv2) Type() sessp.Tfcall { return sessp.TRwatchv2 }
func (Tcreate) Type() sessp.Tfcall  { return sessp.TTcreate }
func (Rcreate) Type() sessp.Tfcall  { return sessp.TRcreate }
func (Rread) Type() sessp.Tfcall    { return sessp.TRread }
func (Rwrite) Type() sessp.Tfcall   { return sessp.TRwrite }
func (Tclunk) Type() sessp.Tfcall   { return sessp.TTclunk }
func (Rclunk) Type() sessp.Tfcall   { return sessp.TRclunk }
func (Tremove) Type() sessp.Tfcall  { return sessp.TTremove }
func (Rremove) Type() sessp.Tfcall  { return sessp.TRremove }
func (Trstat) Type() sessp.Tfcall   { return sessp.TTstat }
func (Twstat) Type() sessp.Tfcall   { return sessp.TTwstat }
func (Rwstat) Type() sessp.Tfcall   { return sessp.TRwstat }

// sigmaP
func (Rrstat) Type() sessp.Tfcall      { return sessp.TRstat }
func (TreadF) Type() sessp.Tfcall      { return sessp.TTreadF }
func (TwriteF) Type() sessp.Tfcall     { return sessp.TTwriteF }
func (Trenameat) Type() sessp.Tfcall   { return sessp.TTrenameat }
func (Rrenameat) Type() sessp.Tfcall   { return sessp.TRrenameat }
func (Tremovefile) Type() sessp.Tfcall { return sessp.TTremovefile }
func (Tgetfile) Type() sessp.Tfcall    { return sessp.TTgetfile }
func (Tputfile) Type() sessp.Tfcall    { return sessp.TTputfile }
func (Tdetach) Type() sessp.Tfcall     { return sessp.TTdetach }
func (Rdetach) Type() sessp.Tfcall     { return sessp.TRdetach }
func (Theartbeat) Type() sessp.Tfcall  { return sessp.TTheartbeat }
func (Rheartbeat) Type() sessp.Tfcall  { return sessp.TRheartbeat }
func (Twriteread) Type() sessp.Tfcall  { return sessp.TTwriteread }
