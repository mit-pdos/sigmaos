package fidclnt

import (
	"fmt"

	np "ulambda/ninep"
	"ulambda/protclnt"
)

//
// Sigma file system API at the level of fids.
//

type FidClnt struct {
	fids *FidMap
	pc   *protclnt.Clnt
}

func MakeFidClnt() *FidClnt {
	fidc := &FidClnt{}
	fidc.fids = mkFidMap()
	fidc.pc = protclnt.MakeClnt()
	return fidc
}

func (fidc *FidClnt) String() string {
	str := fmt.Sprintf("Fsclnt fid table:\n")
	str += fmt.Sprintf("fids %v\n", fidc.fids)
	return str
}

func (fidc *FidClnt) ReadSeqNo() np.Tseqno {
	return fidc.pc.ReadSeqNo()
}

func (fidc *FidClnt) Exit() {
	fidc.pc.Exit()
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

func (fidc *FidClnt) Insert(fid np.Tfid, path *Channel) {
	fidc.fids.insert(fid, path)
}

func (fidc *FidClnt) Clunk(fid np.Tfid) *np.Err {
	err := fidc.fids.lookup(fid).pc.Clunk(fid)
	if err != nil {
		return err
	}
	fidc.fids.free(fid)
	return nil
}

func (fidc *FidClnt) Attach(uname string, server []string, path, tree string) (np.Tfid, *np.Err) {
	fid := fidc.allocFid()
	reply, err := fidc.pc.Attach(server, uname, fid, np.Split(tree))
	if err != nil {
		fidc.freeFid(fid)
		return np.NoFid, err
	}
	ch := fidc.pc.MakeProtClnt(server)
	fidc.fids.insert(fid, makeChannel(ch, uname, np.Split(path), []np.Tqid{reply.Qid}))
	return fid, nil
}

func (fidc *FidClnt) Walk(fid np.Tfid, path []string) (np.Tfid, []string, *np.Err) {
	nfid := fidc.allocFid()
	reply, err := fidc.Lookup(fid).pc.Walk(fid, nfid, path)
	if err != nil {
		fidc.freeFid(nfid)
		return np.NoFid, path, err
	}
	channel := fidc.Lookup(fid).Copy()
	channel.AddN(reply.Qids, path)
	fidc.Insert(nfid, channel)
	return nfid, path[len(reply.Qids):], nil
}

func (fidc *FidClnt) Clone(fid np.Tfid) (np.Tfid, *np.Err) {
	nfid, _, err := fidc.Walk(fid, nil)
	return nfid, err
}

func (fidc *FidClnt) Create(fid np.Tfid, name string, perm np.Tperm, mode np.Tmode) (np.Tfid, *np.Err) {
	reply, err := fidc.fids.lookup(fid).pc.Create(fid, name, perm, mode)
	if err != nil {
		return np.NoFid, err
	}
	fidc.fids.lookup(fid).add(name, reply.Qid)
	return fid, nil
}

func (fidc *FidClnt) Open(fid np.Tfid, mode np.Tmode) (np.Tqid, *np.Err) {
	reply, err := fidc.fids.lookup(fid).pc.Open(fid, mode)
	if err != nil {
		return np.Tqid{}, err
	}
	return reply.Qid, nil
}

func (fidc *FidClnt) Watch(fid np.Tfid, path []string, version np.TQversion) *np.Err {
	return fidc.fids.lookup(fid).pc.Watch(fid, nil, version)
}

func (fidc *FidClnt) Wstat(fid np.Tfid, st *np.Stat) *np.Err {
	_, err := fidc.fids.lookup(fid).pc.Wstat(fid, st)
	return err
}

func (fidc *FidClnt) Renameat(fid np.Tfid, o string, fid1 np.Tfid, n string) *np.Err {
	if fidc.fids.lookup(fid).pc != fidc.fids.lookup(fid1).pc {
		return np.MkErr(np.TErrInval, "paths at different servers")
	}
	_, err := fidc.fids.lookup(fid).pc.Renameat(fid, o, fid1, n)
	return err
}

func (fidc *FidClnt) Remove(fid np.Tfid) *np.Err {
	return fidc.fids.lookup(fid).pc.Remove(fid)
}

func (fidc *FidClnt) RemoveFile(fid np.Tfid, wnames []string, resolve bool) *np.Err {
	return fidc.fids.lookup(fid).pc.RemoveFile(fid, wnames, resolve)
}

func (fidc *FidClnt) Stat(fid np.Tfid) (*np.Stat, *np.Err) {
	reply, err := fidc.fids.lookup(fid).pc.Stat(fid)
	if err != nil {
		return nil, err
	}
	return &reply.Stat, nil
}

func (fidc *FidClnt) Read(fid np.Tfid, off np.Toffset, cnt np.Tsize) ([]byte, *np.Err) {
	//p := fidc.fids[fdst.fid]
	//version := p.lastqid().Version
	//v := fdst.mode&np.OVERSION == np.OVERSION
	reply, err := fidc.fids.lookup(fid).pc.Read(fid, off, cnt)
	if err != nil {
		return nil, err
	}
	return reply.Data, nil
}

func (fidc *FidClnt) Write(fid np.Tfid, off np.Toffset, data []byte) (np.Tsize, *np.Err) {
	reply, err := fidc.fids.lookup(fid).pc.Write(fid, off, data)
	if err != nil {
		return 0, err
	}
	return reply.Count, nil
}

func (fidc *FidClnt) GetFile(fid np.Tfid, path []string, mode np.Tmode, off np.Toffset, cnt np.Tsize, resolve bool) ([]byte, *np.Err) {
	reply, err := fidc.fids.lookup(fid).pc.GetFile(fid, path, mode, off, cnt, false)
	if err != nil {
		return nil, err
	}
	return reply.Data, err
}

func (fidc *FidClnt) SetFile(fid np.Tfid, path []string, mode np.Tmode, off np.Toffset, data []byte, resolve bool) (np.Tsize, *np.Err) {
	reply, err := fidc.fids.lookup(fid).pc.SetFile(fid, path, mode, off, data, false)
	if err != nil {
		return 0, err
	}
	return reply.Count, nil
}

func (fidc *FidClnt) PutFile(fid np.Tfid, path []string, mode np.Tmode, perm np.Tperm, off np.Toffset, data []byte) (np.Tsize, *np.Err) {
	reply, err := fidc.fids.lookup(fid).pc.PutFile(fid, path, mode, perm, off, data)
	if err != nil {
		return 0, err
	}
	return reply.Count, nil
}

func (fidc *FidClnt) MkFence(fid np.Tfid) (np.Tfence, *np.Err) {
	reply, err := fidc.fids.lookup(fid).pc.MkFence(fid)
	if err != nil {
		return np.Tfence{}, err
	}
	return reply.Fence, nil
}

func (fidc *FidClnt) RegisterFence(fence np.Tfence, fid np.Tfid) *np.Err {
	return fidc.fids.lookup(fid).pc.RegisterFence(fence, fid)
}

func (fidc *FidClnt) DeregisterFence(fence np.Tfence, fid np.Tfid) *np.Err {
	return fidc.fids.lookup(fid).pc.DeregisterFence(fence, fid)
}

func (fidc *FidClnt) RmFence(fence np.Tfence, fid np.Tfid) *np.Err {
	return fidc.fids.lookup(fid).pc.RmFence(fence, fid)
}
