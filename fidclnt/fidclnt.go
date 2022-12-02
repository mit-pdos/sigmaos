package fidclnt

import (
	"fmt"

	"sigmaos/fcall"
	"sigmaos/path"
	"sigmaos/protclnt"
	np "sigmaos/sigmap"
)

//
// Sigma file system API at the level of fids.
//

type FidClnt struct {
	fids *FidMap
	pc   *protclnt.Clnt
	ft   *FenceTable
}

func MakeFidClnt() *FidClnt {
	fidc := &FidClnt{}
	fidc.fids = mkFidMap()
	fidc.pc = protclnt.MakeClnt()
	fidc.ft = MakeFenceTable()
	return fidc
}

func (fidc *FidClnt) String() string {
	str := fmt.Sprintf("Fsclnt fid table %p:\n%v", fidc, fidc.fids)
	return str
}

func (fidc *FidClnt) Len() int {
	return len(fidc.fids.fids)
}

func (fidc *FidClnt) FenceDir(path string, f np.Tfence) *fcall.Err {
	return fidc.ft.Insert(path, f)
}

func (fidc *FidClnt) ReadSeqNo() np.Tseqno {
	return fidc.pc.ReadSeqNo()
}

func (fidc *FidClnt) Exit() *fcall.Err {
	return fidc.pc.Exit()
}

func (fidc *FidClnt) allocFid() np.Tfid {
	return fidc.fids.allocFid()
}

func (fidc *FidClnt) freeFid(np np.Tfid) {
	// not implemented
}

func (fidc *FidClnt) Free(fid np.Tfid) {
	fidc.fids.free(fid)
}

func (fidc *FidClnt) Lookup(fid np.Tfid) *Channel {
	return fidc.fids.lookup(fid)
}

func (fidc *FidClnt) Qid(fid np.Tfid) *np.Tqid {
	return fidc.Lookup(fid).Lastqid()
}

func (fidc *FidClnt) Qids(fid np.Tfid) []*np.Tqid {
	return fidc.Lookup(fid).qids
}

func (fidc *FidClnt) Path(fid np.Tfid) path.Path {
	return fidc.Lookup(fid).Path()
}

func (fidc *FidClnt) Insert(fid np.Tfid, path *Channel) {
	fidc.fids.insert(fid, path)
}

func (fidc *FidClnt) Clunk(fid np.Tfid) *fcall.Err {
	err := fidc.fids.lookup(fid).pc.Clunk(fid)
	if err != nil {
		return err
	}
	fidc.fids.free(fid)
	return nil
}

func (fidc *FidClnt) Attach(uname string, addrs []string, pn, tree string) (np.Tfid, *fcall.Err) {
	fid := fidc.allocFid()
	reply, err := fidc.pc.Attach(addrs, uname, fid, path.Split(tree))
	if err != nil {
		fidc.freeFid(fid)
		return np.NoFid, err
	}
	pc := fidc.pc.MakeProtClnt(addrs)
	fidc.fids.insert(fid, makeChannel(pc, uname, path.Split(pn), []*np.Tqid{reply.Qid}))
	return fid, nil
}

func (fidc *FidClnt) Detach(fid np.Tfid) *fcall.Err {
	ch := fidc.fids.lookup(fid)
	if ch == nil {
		return fcall.MkErr(fcall.TErrUnreachable, "detach")
	}
	if err := ch.pc.Detach(); err != nil {
		return err
	}
	return nil
}

// Walk returns the fid it walked to (which maybe fid) and the
// remaining path left to be walked (which maybe the original path).
func (fidc *FidClnt) Walk(fid np.Tfid, path []string) (np.Tfid, []string, *fcall.Err) {
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
func (fidc *FidClnt) Clone(fid np.Tfid) (np.Tfid, *fcall.Err) {
	nfid := fidc.allocFid()
	channel := fidc.Lookup(fid)
	if channel == nil {
		return np.NoFid, fcall.MkErr(fcall.TErrUnreachable, "clone")
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

func (fidc *FidClnt) Create(fid np.Tfid, name string, perm np.Tperm, mode np.Tmode) (np.Tfid, *fcall.Err) {
	reply, err := fidc.fids.lookup(fid).pc.Create(fid, name, perm, mode)
	if err != nil {
		return np.NoFid, err
	}
	fidc.fids.lookup(fid).add(name, &reply.Qid)
	return fid, nil
}

func (fidc *FidClnt) Open(fid np.Tfid, mode np.Tmode) (np.Tqid, *fcall.Err) {
	reply, err := fidc.fids.lookup(fid).pc.Open(fid, mode)
	if err != nil {
		return np.Tqid{}, err
	}
	return reply.Qid, nil
}

func (fidc *FidClnt) Watch(fid np.Tfid) *fcall.Err {
	return fidc.fids.lookup(fid).pc.Watch(fid)
}

func (fidc *FidClnt) Wstat(fid np.Tfid, st *np.Stat) *fcall.Err {
	f := fidc.ft.Lookup(fidc.fids.lookup(fid).Path())
	_, err := fidc.fids.lookup(fid).pc.WstatF(fid, st, f)
	return err
}

func (fidc *FidClnt) Renameat(fid np.Tfid, o string, fid1 np.Tfid, n string) *fcall.Err {
	f := fidc.ft.Lookup(fidc.fids.lookup(fid).Path())
	if fidc.fids.lookup(fid).pc != fidc.fids.lookup(fid1).pc {
		return fcall.MkErr(fcall.TErrInval, "paths at different servers")
	}
	_, err := fidc.fids.lookup(fid).pc.Renameat(fid, o, fid1, n, f)
	return err
}

func (fidc *FidClnt) Remove(fid np.Tfid) *fcall.Err {
	f := fidc.ft.Lookup(fidc.fids.lookup(fid).Path())
	return fidc.fids.lookup(fid).pc.RemoveF(fid, f)
}

func (fidc *FidClnt) RemoveFile(fid np.Tfid, wnames []string, resolve bool) *fcall.Err {
	ch := fidc.fids.lookup(fid)
	if ch == nil {
		return fcall.MkErr(fcall.TErrUnreachable, "getfile")
	}
	f := fidc.ft.Lookup(ch.Path().AppendPath(wnames))
	return ch.pc.RemoveFile(fid, wnames, resolve, f)
}

func (fidc *FidClnt) Stat(fid np.Tfid) (*np.Stat, *fcall.Err) {
	reply, err := fidc.fids.lookup(fid).pc.Stat(fid)
	if err != nil {
		return nil, err
	}
	return reply.Stat, nil
}

func (fidc *FidClnt) ReadV(fid np.Tfid, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *fcall.Err) {
	f := fidc.ft.Lookup(fidc.fids.lookup(fid).Path())
	reply, err := fidc.fids.lookup(fid).pc.ReadVF(fid, off, cnt, f, v)
	if err != nil {
		return nil, err
	}
	return reply.Data, nil
}

// Unfenced read
func (fidc *FidClnt) ReadVU(fid np.Tfid, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *fcall.Err) {
	reply, err := fidc.fids.lookup(fid).pc.ReadVF(fid, off, cnt, np.MakeFenceNull(), v)
	if err != nil {
		return nil, err
	}
	return reply.Data, nil
}

func (fidc *FidClnt) WriteV(fid np.Tfid, off np.Toffset, data []byte, v np.TQversion) (np.Tsize, *fcall.Err) {
	f := fidc.ft.Lookup(fidc.fids.lookup(fid).Path())
	reply, err := fidc.fids.lookup(fid).pc.WriteVF(fid, off, f, v, data)
	if err != nil {
		return 0, err
	}
	return reply.Count, nil
}

func (fidc *FidClnt) WriteRead(fid np.Tfid, data []byte) ([]byte, *fcall.Err) {
	ch := fidc.fids.lookup(fid)
	if ch == nil {
		return nil, fcall.MkErr(fcall.TErrUnreachable, "WriteRead")
	}
	reply, err := fidc.fids.lookup(fid).pc.WriteRead(fid, data)
	if err != nil {
		return nil, err
	}
	return reply.Data, nil
}

func (fidc *FidClnt) GetFile(fid np.Tfid, path []string, mode np.Tmode, off np.Toffset, cnt np.Tsize, resolve bool) ([]byte, *fcall.Err) {
	ch := fidc.fids.lookup(fid)
	if ch == nil {
		return nil, fcall.MkErr(fcall.TErrUnreachable, "getfile")
	}
	f := fidc.ft.Lookup(ch.Path().AppendPath(path))
	reply, err := ch.pc.GetFile(fid, path, mode, off, cnt, resolve, f)
	if err != nil {
		return nil, err
	}
	return reply.Data, err
}

func (fidc *FidClnt) SetFile(fid np.Tfid, path []string, mode np.Tmode, off np.Toffset, data []byte, resolve bool) (np.Tsize, *fcall.Err) {
	ch := fidc.fids.lookup(fid)
	if ch == nil {
		return 0, fcall.MkErr(fcall.TErrUnreachable, "getfile")
	}
	f := fidc.ft.Lookup(ch.Path().AppendPath(path))
	reply, err := ch.pc.SetFile(fid, path, mode, off, resolve, f, data)
	if err != nil {
		return 0, err
	}
	return reply.Count, nil
}

func (fidc *FidClnt) PutFile(fid np.Tfid, path []string, mode np.Tmode, perm np.Tperm, off np.Toffset, data []byte) (np.Tsize, *fcall.Err) {
	ch := fidc.fids.lookup(fid)
	if ch == nil {
		return 0, fcall.MkErr(fcall.TErrUnreachable, "putfile")
	}
	f := fidc.ft.Lookup(ch.Path().AppendPath(path))
	reply, err := ch.pc.PutFile(fid, path, mode, perm, off, f, data)
	if err != nil {
		return 0, err
	}
	return reply.Count, nil
}
