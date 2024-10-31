// Package fidclnt implements SigmaOS API using fid's.  Fidclnt's
// session manager ([sessclnt]) makes sessions to the server
// associated with a fid: that is, fids that have the same remote
// server associated with them share a single session to that server.
//
// Several instances of [pathclnt] may share a single fidclnt, which
// causes a session to a server to be shared among those pathclnt's.
package fidclnt

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/netproxyclnt"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/protclnt"
	"sigmaos/serr"
	"sigmaos/sessclnt"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type FidClnt struct {
	mu     sync.Mutex
	fids   *FidMap
	refcnt int
	sm     *sessclnt.Mgr
	npc    *netproxyclnt.NetProxyClnt
}

func NewFidClnt(pe *proc.ProcEnv, npc *netproxyclnt.NetProxyClnt) *FidClnt {
	return &FidClnt{
		fids:   newFidMap(),
		refcnt: 1,
		sm:     sessclnt.NewMgr(pe, npc),
		npc:    npc,
	}
}

func (fidc *FidClnt) String() string {
	str := fmt.Sprintf("{fids %v}", fidc.fids)
	return str
}

func (fidc *FidClnt) GetNetProxyClnt() *netproxyclnt.NetProxyClnt {
	return fidc.npc
}

func (fidc *FidClnt) NewClnt() {
	fidc.mu.Lock()
	defer fidc.mu.Unlock()
	fidc.refcnt++
}

// Close all sessions
func (fidc *FidClnt) closeSess() error {
	var err error
	scs := fidc.sm.SessClnts()
	for _, sc := range scs {
		if r := sc.Close(); r != nil {
			err = r
		}
	}
	return err
}

func (fidc *FidClnt) Close() error {
	fidc.mu.Lock()
	defer fidc.mu.Unlock()

	if fidc.refcnt <= 0 {
		db.DPrintf(db.ALWAYS, "FidClnt already closed\n")
		return nil // XXX maybe return error
	}
	fidc.refcnt--
	db.DPrintf(db.FIDCLNT, "FidClnt refcnt %d\n", fidc.refcnt)
	if fidc.refcnt == 0 {
		fidc.closeSess()
	}
	return nil
}

func (fidc *FidClnt) Len() int {
	return len(fidc.fids.fids)
}

func (fidc *FidClnt) allocFid() sp.Tfid {
	return fidc.fids.allocFid()
}

func (fidc *FidClnt) freeFid(np sp.Tfid) {
	// XXX not implemented
}

func (fidc *FidClnt) Free(fid sp.Tfid) {
	fidc.fids.free(fid)
}

func (fidc *FidClnt) DisconnectAll(fid sp.Tfid) {
	fidc.fids.disconnect(fid)
}

func (fidc *FidClnt) Lookup(fid sp.Tfid) *Channel {
	return fidc.fids.lookup(fid)
}

func (fidc *FidClnt) Qid(fid sp.Tfid) *sp.Tqid {
	return fidc.Lookup(fid).Lastqid()
}

func (fidc *FidClnt) Qids(fid sp.Tfid) []*sp.Tqid {
	return fidc.Lookup(fid).Qids()
}

func (fidc *FidClnt) Insert(fid sp.Tfid, path *Channel) {
	fidc.fids.insert(fid, path)
}

func (fidc *FidClnt) Clunk(fid sp.Tfid) error {
	ch := fidc.Lookup(fid)
	if ch == nil {
		return serr.NewErr(serr.TErrUnreachable, "Clunk")
	}
	if err := ch.pc.Clunk(fid); err != nil {
		return err
	}
	fidc.fids.free(fid)
	return nil
}

func (fidc *FidClnt) Attach(secrets map[string]*sp.SecretProto, cid sp.TclntId, ep *sp.Tendpoint, pn path.Tpathname, tree string) (sp.Tfid, *serr.Err) {
	s := time.Now()
	fid := fidc.allocFid()
	pc := protclnt.NewProtClnt(ep, fidc.sm)
	reply, err := pc.Attach(secrets, cid, fid, path.Split(tree))
	if err != nil {
		db.DPrintf(db.FIDCLNT_ERR, "Error attach %v: %v", ep, err)
		fidc.freeFid(fid)
		return sp.NoFid, err
	}
	fidc.fids.insert(fid, newChannel(pc, []*sp.Tqid{sp.NewTqid(reply.Qid)}))
	db.DPrintf(db.ATTACH_LAT, "%v: attach %v pn %q tree %q lat %v\n", cid, ep, pn, tree, time.Since(s))
	return fid, nil
}

func (fidc *FidClnt) Detach(fid sp.Tfid, cid sp.TclntId) *serr.Err {
	ch := fidc.Lookup(fid)
	if ch == nil {
		return serr.NewErr(serr.TErrUnreachable, "Detach")
	}
	if err := ch.pc.Detach(cid); err != nil {
		return err
	}
	return nil
}

// Walk returns the fid it walked to (which maybe fid) and the
// remaining path left to be walked (which maybe the original path).
func (fidc *FidClnt) Walk(fid sp.Tfid, path []string) (sp.Tfid, []string, *serr.Err) {
	nfid := fidc.allocFid()
	ch := fidc.Lookup(fid)
	if ch == nil {
		return sp.NoFid, nil, serr.NewErr(serr.TErrUnreachable, "Walk")
	}
	reply, err := ch.pc.Walk(fid, nfid, path)
	if err != nil {
		fidc.freeFid(nfid)
		return fid, path, err
	}
	channel := ch.Copy()
	channel.AddQids(reply.Qids)
	fidc.Insert(nfid, channel)
	return nfid, path[len(reply.Qids):], nil
}

// A defensive version of walk because fid is shared among several
// threads (it comes out the endpoint table) and one thread may free the
// fid while another thread is using it.
func (fidc *FidClnt) Clone(fid sp.Tfid) (sp.Tfid, *serr.Err) {
	nfid := fidc.allocFid()
	ch := fidc.Lookup(fid)
	if ch == nil {
		return sp.NoFid, serr.NewErr(serr.TErrUnreachable, "Clone")
	}
	_, err := ch.pc.Walk(fid, nfid, path.Tpathname{})
	if err != nil {
		fidc.freeFid(nfid)
		return fid, err
	}
	ch = ch.Copy()
	fidc.Insert(nfid, ch)
	return nfid, err
}

func (fidc *FidClnt) Create(fid sp.Tfid, name string, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId, f sp.Tfence) (sp.Tfid, *serr.Err) {
	db.DPrintf(db.FIDCLNT, "Create %v name %v", fid, name)
	ch := fidc.Lookup(fid)
	if ch == nil {
		return sp.NoFid, serr.NewErr(serr.TErrUnreachable, "Create")
	}
	reply, err := ch.pc.Create(fid, name, perm, mode, lid, f)
	db.DPrintf(db.FIDCLNT, "Create done %v name %v err %v", fid, name, err)
	if err != nil {
		return sp.NoFid, err
	}
	ch.addQid(reply.Qid)
	return fid, nil
}

func (fidc *FidClnt) Open(fid sp.Tfid, mode sp.Tmode) (*sp.Tqid, *serr.Err) {
	ch := fidc.Lookup(fid)
	if ch == nil {
		return nil, serr.NewErr(serr.TErrUnreachable, "Open")
	}
	reply, err := ch.pc.Open(fid, mode)
	if err != nil {
		return nil, err
	}
	return sp.NewTqid(reply.Qid), nil
}

func (fidc *FidClnt) Watch(fid sp.Tfid) *serr.Err {
	ch := fidc.Lookup(fid)
	if ch == nil {
		return serr.NewErr(serr.TErrUnreachable, "Watch")
	}
	return ch.pc.Watch(fid)
}

func (fidc *FidClnt) Wstat(fid sp.Tfid, st *sp.Stat, f *sp.Tfence) *serr.Err {
	ch := fidc.Lookup(fid)
	if ch == nil {
		return serr.NewErr(serr.TErrUnreachable, "Wstat")
	}
	_, err := ch.pc.WstatF(fid, st, f)
	return err
}

func (fidc *FidClnt) Renameat(fid sp.Tfid, o string, fid1 sp.Tfid, n string, f *sp.Tfence) *serr.Err {
	ch := fidc.Lookup(fid)
	if ch == nil {
		return serr.NewErr(serr.TErrUnreachable, "Renameat")
	}
	ch1 := fidc.Lookup(fid1)
	if ch1 == nil {
		return serr.NewErr(serr.TErrUnreachable, "Renameat1")
	}
	if ch.pc != ch1.pc {
		return serr.NewErr(serr.TErrInval, "paths at different servers")
	}
	_, err := ch.pc.Renameat(fid, o, fid1, n, f)
	return err
}

func (fidc *FidClnt) Remove(fid sp.Tfid, f *sp.Tfence) *serr.Err {
	ch := fidc.Lookup(fid)
	if ch == nil {
		return serr.NewErr(serr.TErrUnreachable, "Remove")
	}
	return ch.pc.RemoveF(fid, f)
}

func (fidc *FidClnt) RemoveFile(fid sp.Tfid, wnames []string, resolve bool, f *sp.Tfence) *serr.Err {
	ch := fidc.Lookup(fid)
	if ch == nil {
		return serr.NewErr(serr.TErrUnreachable, "RemoveFile")
	}
	return ch.pc.RemoveFile(fid, wnames, resolve, f)
}

func (fidc *FidClnt) Stat(fid sp.Tfid) (*sp.Stat, *serr.Err) {
	ch := fidc.Lookup(fid)
	if ch == nil {
		return nil, serr.NewErr(serr.TErrUnreachable, "Stat")
	}
	reply, err := ch.pc.Stat(fid)
	if err != nil {
		return nil, err
	}
	return sp.NewStatProto(reply.Stat), nil
}

func (fidc *FidClnt) ReadF(fid sp.Tfid, off sp.Toffset, b []byte, f *sp.Tfence) (sp.Tsize, error) {
	ch := fidc.Lookup(fid)
	if ch == nil {
		return 0, serr.NewErr(serr.TErrUnreachable, "ReadF")
	}
	cnt, err := ch.pc.ReadF(fid, off, b, f)
	if err != nil {
		return 0, err
	}
	return cnt, nil
}

func (fidc *FidClnt) PreadRdr(fid sp.Tfid, off sp.Toffset, sz sp.Tsize) (io.ReadCloser, error) {
	b := make([]byte, sz)
	ch := fidc.Lookup(fid)
	if ch == nil {
		return nil, serr.NewErr(serr.TErrUnreachable, "ReadF")
	}
	cnt, err := ch.pc.ReadF(fid, off, b, sp.NullFence())
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(b[0:cnt])), nil
}

func (fidc *FidClnt) WriteF(fid sp.Tfid, off sp.Toffset, data []byte, f *sp.Tfence) (sp.Tsize, error) {
	ch := fidc.Lookup(fid)
	if ch == nil {
		return 0, serr.NewErr(serr.TErrUnreachable, "WriteF")
	}
	reply, err := ch.pc.WriteF(fid, off, f, data)
	if err != nil {
		return 0, err
	}
	return reply.Tcount(), nil
}

func (fidc *FidClnt) WriteRead(fid sp.Tfid, iniov sessp.IoVec, outiov sessp.IoVec) *serr.Err {
	ch := fidc.Lookup(fid)
	if ch == nil {
		return serr.NewErr(serr.TErrUnreachable, "WriteRead")
	}
	err := fidc.fids.lookup(fid).pc.WriteRead(fid, iniov, outiov)
	if err != nil {
		return err
	}
	return nil
}

func (fidc *FidClnt) GetFile(fid sp.Tfid, path []string, mode sp.Tmode, off sp.Toffset, cnt sp.Tsize, resolve bool, f *sp.Tfence) ([]byte, *serr.Err) {
	ch := fidc.Lookup(fid)
	if ch == nil {
		return nil, serr.NewErr(serr.TErrUnreachable, "GetFile")
	}
	data, err := ch.pc.GetFile(fid, path, mode, off, cnt, resolve, f)
	if err != nil {
		return nil, err
	}
	return data, err
}

func (fidc *FidClnt) PutFile(fid sp.Tfid, path []string, mode sp.Tmode, perm sp.Tperm, off sp.Toffset, data []byte, resolve bool, lid sp.TleaseId, f *sp.Tfence) (sp.Tsize, *serr.Err) {
	ch := fidc.Lookup(fid)
	if ch == nil {
		return 0, serr.NewErr(serr.TErrUnreachable, "PutFile")
	}
	reply, err := ch.pc.PutFile(fid, path, mode, perm, off, resolve, f, data, lid)
	if err != nil {
		return 0, err
	}
	return reply.Tcount(), nil
}
