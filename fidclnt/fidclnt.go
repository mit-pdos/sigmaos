// Package fidclnt implements SigmaOS API using fid's
package fidclnt

import (
	"fmt"
	"sync"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/protclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type FidClnt struct {
	mu     sync.Mutex
	fids   *FidMap
	pc     *protclnt.Clnt
	refcnt int
}

func NewFidClnt(clntnet string) *FidClnt {
	fidc := &FidClnt{}
	fidc.fids = newFidMap()
	fidc.pc = protclnt.NewClnt(clntnet)
	fidc.refcnt = 1
	return fidc
}

func (fidc *FidClnt) String() string {
	str := fmt.Sprintf("Fsclnt fid table %p:\n%v", fidc, fidc.fids)
	return str
}

func (fidc *FidClnt) NewClnt() {
	fidc.mu.Lock()
	defer fidc.mu.Unlock()
	fidc.refcnt++
}

func (fidc *FidClnt) Close() error {
	fidc.mu.Lock()
	defer fidc.mu.Unlock()

	if fidc.refcnt <= 0 {
		db.DPrintf(db.ALWAYS, "FidClnt already closed\n")
		return nil // XXX maybe return error
	}
	fidc.refcnt--
	db.DPrintf(db.ALWAYS, "FidClnt refcnt %d\n", fidc.refcnt)
	if fidc.refcnt == 0 {
		return fidc.pc.Close()
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

func (fidc *FidClnt) Lookup(fid sp.Tfid) *Channel {
	return fidc.fids.lookup(fid)
}

func (fidc *FidClnt) Qid(fid sp.Tfid) *sp.Tqid {
	return fidc.Lookup(fid).Lastqid()
}

func (fidc *FidClnt) Qids(fid sp.Tfid) []*sp.Tqid {
	return fidc.Lookup(fid).qids
}

func (fidc *FidClnt) Path(fid sp.Tfid) path.Path {
	return fidc.Lookup(fid).Path()
}

func (fidc *FidClnt) Insert(fid sp.Tfid, path *Channel) {
	fidc.fids.insert(fid, path)
}

func (fidc *FidClnt) Clunk(fid sp.Tfid) *serr.Err {
	err := fidc.fids.lookup(fid).pc.Clunk(fid)
	if err != nil {
		return err
	}
	fidc.fids.free(fid)
	return nil
}

func (fidc *FidClnt) Attach(uname sp.Tuname, cid sp.TclntId, addrs sp.Taddrs, pn, tree string) (sp.Tfid, *serr.Err) {
	fid := fidc.allocFid()
	reply, err := fidc.pc.Attach(addrs, uname, cid, fid, path.Split(tree))
	if err != nil {
		db.DPrintf(db.FIDCLNT_ERR, "Error attach %v: %v", addrs, err)
		fidc.freeFid(fid)
		return sp.NoFid, err
	}
	pc := fidc.pc.NewProtClnt(addrs)
	fidc.fids.insert(fid, newChannel(pc, uname, path.Split(pn), []*sp.Tqid{reply.Qid}))
	return fid, nil
}

func (fidc *FidClnt) Detach(fid sp.Tfid, cid sp.TclntId) *serr.Err {
	ch := fidc.fids.lookup(fid)
	if ch == nil {
		return serr.NewErr(serr.TErrUnreachable, "detach")
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
	reply, err := fidc.Lookup(fid).pc.Walk(fid, nfid, path)
	if err != nil {
		fidc.freeFid(nfid)
		return fid, path, err
	}
	channel := fidc.Lookup(fid).Copy()
	channel.AddN(reply.Qids, path)
	fidc.Insert(nfid, channel)
	return nfid, path[len(reply.Qids):], nil
}

// A defensive version of walk because fid is shared among several
// threads (it comes out the mount table) and one thread may free the
// fid while another thread is using it.
func (fidc *FidClnt) Clone(fid sp.Tfid) (sp.Tfid, *serr.Err) {
	nfid := fidc.allocFid()
	channel := fidc.Lookup(fid)
	if channel == nil {
		return sp.NoFid, serr.NewErr(serr.TErrUnreachable, "clone")
	}
	_, err := channel.pc.Walk(fid, nfid, path.Path{})
	if err != nil {
		fidc.freeFid(nfid)
		return fid, err
	}
	channel = channel.Copy()
	fidc.Insert(nfid, channel)
	return nfid, err
}

func (fidc *FidClnt) Create(fid sp.Tfid, name string, perm sp.Tperm, mode sp.Tmode, lid sp.TleaseId, f sp.Tfence) (sp.Tfid, *serr.Err) {
	db.DPrintf(db.FIDCLNT, "Create %v name %v", fid, name)
	reply, err := fidc.fids.lookup(fid).pc.Create(fid, name, perm, mode, lid, f)
	db.DPrintf(db.FIDCLNT, "Create done %v name %v err %v", fid, name, err)
	if err != nil {
		return sp.NoFid, err
	}
	fidc.fids.lookup(fid).add(name, reply.Qid)
	return fid, nil
}

func (fidc *FidClnt) Open(fid sp.Tfid, mode sp.Tmode) (*sp.Tqid, *serr.Err) {
	reply, err := fidc.fids.lookup(fid).pc.Open(fid, mode)
	if err != nil {
		return nil, err
	}
	return reply.Qid, nil
}

func (fidc *FidClnt) Watch(fid sp.Tfid) *serr.Err {
	return fidc.fids.lookup(fid).pc.Watch(fid)
}

func (fidc *FidClnt) Wstat(fid sp.Tfid, st *sp.Stat, f *sp.Tfence) *serr.Err {
	_, err := fidc.fids.lookup(fid).pc.WstatF(fid, st, f)
	return err
}

func (fidc *FidClnt) Renameat(fid sp.Tfid, o string, fid1 sp.Tfid, n string, f *sp.Tfence) *serr.Err {
	if fidc.fids.lookup(fid).pc != fidc.fids.lookup(fid1).pc {
		return serr.NewErr(serr.TErrInval, "paths at different servers")
	}
	_, err := fidc.fids.lookup(fid).pc.Renameat(fid, o, fid1, n, f)
	return err
}

func (fidc *FidClnt) Remove(fid sp.Tfid, f *sp.Tfence) *serr.Err {
	return fidc.fids.lookup(fid).pc.RemoveF(fid, f)
}

func (fidc *FidClnt) RemoveFile(fid sp.Tfid, wnames []string, resolve bool, f *sp.Tfence) *serr.Err {
	ch := fidc.fids.lookup(fid)
	if ch == nil {
		return serr.NewErr(serr.TErrUnreachable, "getfile")
	}
	return ch.pc.RemoveFile(fid, wnames, resolve, f)
}

func (fidc *FidClnt) Stat(fid sp.Tfid) (*sp.Stat, *serr.Err) {
	reply, err := fidc.fids.lookup(fid).pc.Stat(fid)
	if err != nil {
		return nil, err
	}
	return reply.Stat, nil
}

func (fidc *FidClnt) ReadF(fid sp.Tfid, off sp.Toffset, cnt sp.Tsize, f *sp.Tfence) ([]byte, *serr.Err) {
	data, err := fidc.fids.lookup(fid).pc.ReadF(fid, off, cnt, f)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (fidc *FidClnt) WriteF(fid sp.Tfid, off sp.Toffset, data []byte, f *sp.Tfence) (sp.Tsize, *serr.Err) {
	reply, err := fidc.fids.lookup(fid).pc.WriteF(fid, off, f, data)
	if err != nil {
		return 0, err
	}
	return reply.Tcount(), nil
}

func (fidc *FidClnt) WriteRead(fid sp.Tfid, data []byte) ([]byte, *serr.Err) {
	ch := fidc.fids.lookup(fid)
	if ch == nil {
		return nil, serr.NewErr(serr.TErrUnreachable, "WriteRead")
	}
	data, err := fidc.fids.lookup(fid).pc.WriteRead(fid, data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (fidc *FidClnt) GetFile(fid sp.Tfid, path []string, mode sp.Tmode, off sp.Toffset, cnt sp.Tsize, resolve bool, f *sp.Tfence) ([]byte, *serr.Err) {
	ch := fidc.fids.lookup(fid)
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
	ch := fidc.fids.lookup(fid)
	if ch == nil {
		return 0, serr.NewErr(serr.TErrUnreachable, "PutFile")
	}
	reply, err := ch.pc.PutFile(fid, path, mode, perm, off, resolve, f, data, lid)
	if err != nil {
		return 0, err
	}
	return reply.Tcount(), nil
}
