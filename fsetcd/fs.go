package fsetcd

import (
	"context"
	"fmt"
	"strconv"

	"go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/sorteddir"
)

type Tstat int
type Tcacheable int

const (
	BOOT sp.Tpath = 0

	TSTAT_NONE Tstat = iota
	TSTAT_STAT

	EPHEMERAL = "t-"
)

func (fs *FsEtcd) path2key(realm sp.Trealm, dei *DirEntInfo) string {
	if dei.Perm.IsEphemeral() {
		return EPHEMERAL + string(realm) + ":" + strconv.FormatUint(uint64(dei.Path), 16)
	}
	return string(realm) + ":" + strconv.FormatUint(uint64(dei.Path), 16)
}

func (fs *FsEtcd) getFile(key string) (*EtcdFile, sp.TQversion, *serr.Err) {
	db.DPrintf(db.FSETCD, "getFile %v\n", key)
	resp, err := fs.Clnt().Get(context.TODO(), key)
	db.DPrintf(db.FSETCD, "getFile %v %v err %v\n", key, resp, err)
	if err != nil {
		return nil, 0, serr.NewErrError(err)
	}
	if len(resp.Kvs) != 1 {
		return nil, 0, serr.NewErr(serr.TErrNotfound, key2path(key))
	}
	nf := newEtcdFile()
	if err := proto.Unmarshal(resp.Kvs[0].Value, nf.EtcdFileProto); err != nil {
		return nil, 0, serr.NewErrError(err)
	}
	db.DPrintf(db.FSETCD, "getFile %v %v\n", key, nf)
	return nf, sp.TQversion(resp.Kvs[0].Version), nil
}

func (fs *FsEtcd) GetFile(dei *DirEntInfo) (*EtcdFile, sp.TQversion, *serr.Err) {
	return fs.getFile(fs.path2key(fs.realm, dei))
}

func (fs *FsEtcd) PutFile(dei *DirEntInfo, nf *EtcdFile, f sp.Tfence) *serr.Err {
	opts := nf.LeaseOpts()
	if b, err := proto.Marshal(nf.EtcdFileProto); err != nil {
		return serr.NewErrError(err)
	} else {
		var cmp []clientv3.Cmp
		if f.PathName == "" {
			cmp = []clientv3.Cmp{
				clientv3.Compare(clientv3.CreateRevision(fs.fencekey), "=", fs.fencerev),
			}
		} else {
			cmp = []clientv3.Cmp{
				clientv3.Compare(clientv3.CreateRevision(f.PathName), "=", int64(f.Epoch)),
			}
		}
		opst := []clientv3.Op{
			clientv3.OpPut(fs.path2key(fs.realm, dei), string(b), opts...),
		}
		opsf := []clientv3.Op{
			clientv3.OpGet(f.Prefix(), opts...),
		}
		resp, err := fs.Clnt().Txn(context.TODO()).If(cmp...).Then(opst...).Else(opsf...).Commit()
		db.DPrintf(db.FSETCD, "PutFile dei %v f %v resp %v err %v\n", dei, f, resp, err)
		if err != nil {
			return serr.NewErrError(err)
		}
		if !resp.Succeeded {
			if len(resp.Responses[0].GetResponseRange().Kvs) != 1 {
				db.DPrintf(db.FENCEFS, "PutFile dei %v f %v resp %v stale\n", dei, f, resp)
				return serr.NewErr(serr.TErrStale, f)
			}
			db.DPrintf(db.ERROR, "PutFile failed dei %v %v\n", dei, resp.Responses[0])
		}

		return nil
	}
}

func (fs *FsEtcd) readDir(dei *DirEntInfo, stat Tstat) (*DirInfo, sp.TQversion, *serr.Err) {
	if de, ok := fs.dc.lookup(dei.Path); ok && (stat == TSTAT_NONE || de.stat == TSTAT_STAT) {
		db.DPrintf(db.FSETCD, "fsetcd.readDir %v\n", de.dir)
		return de.dir, de.v, nil
	}
	dir, v, err := fs.readDirEtcd(dei, stat)
	if err != nil {
		return nil, v, err
	}
	fs.dc.insert(dei.Path, &dcEntry{dir, v, stat})
	return dir, v, nil
}

// If stat is TSTAT_STAT, stat every entry in the directory.
func (fs *FsEtcd) readDirEtcd(dei *DirEntInfo, stat Tstat) (*DirInfo, sp.TQversion, *serr.Err) {
	db.DPrintf(db.FSETCD, "readDirEtcd %v %v\n", dei.Path, stat)
	nf, v, err := fs.GetFile(dei)
	if err != nil {
		return nil, 0, err
	}
	dir, err := UnmarshalDir(nf.Data)
	if err != nil {
		return nil, 0, err
	}
	dents := sorteddir.NewSortedDir[string, *DirEntInfo]()
	update := false
	for _, e := range dir.Ents {
		if e.Name == "." {
			dents.Insert(e.Name, newDirEntInfo(nf, e.Tpath(), e.Tperm()))
		} else {
			di := newDirEntInfoP(e.Tpath(), e.Tperm())
			if stat == TSTAT_STAT { // if STAT, get file info
				nf, _, err := fs.GetFile(di)
				if err != nil {
					db.DPrintf(db.FSETCD, "readDir: expired %v %v err %v\n", e.Name, e.Tperm(), err)
					update = true
					continue
				}
				di.Nf = nf
				dents.Insert(e.Name, di)
			} else {
				dents.Insert(e.Name, newDirEntInfoP(e.Tpath(), e.Tperm()))
			}
		}
	}
	di := &DirInfo{dents, nf.Tperm()}
	if update {
		if err := fs.updateDir(dei, di, v); err != nil {
			if err.IsErrVersion() {
				// retry?
				return di, v, nil
			}
			return nil, 0, err
		}
		v = v + 1
	}
	return di, v, nil
}

func (fs *FsEtcd) updateDir(dei *DirEntInfo, dir *DirInfo, v sp.TQversion) *serr.Err {
	d1, r := marshalDirInfo(dir)
	if r != nil {
		return r
	}
	// Update directory if directory hasn't changed.
	cmp := []clientv3.Cmp{
		clientv3.Compare(clientv3.CreateRevision(fs.fencekey), "=", fs.fencerev),
		clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, dei)), "=", int64(v))}
	ops := []clientv3.Op{
		clientv3.OpPut(fs.path2key(fs.realm, dei), string(d1))}
	ops1 := []clientv3.Op{
		clientv3.OpGet(fs.fencekey),
		clientv3.OpGet(fs.path2key(fs.realm, dei))}
	resp, err := fs.Clnt().Txn(context.TODO()).If(cmp...).Then(ops...).Else(ops1...).Commit()
	db.DPrintf(db.FSETCD, "updateDir %v %v %v %v err %v\n", dei.Path, dir, v, resp, err)
	if err != nil {
		return serr.NewErrError(err)
	}
	if !resp.Succeeded {
		if len(resp.Responses[0].GetResponseRange().Kvs) == 1 &&
			resp.Responses[0].GetResponseRange().Kvs[0].CreateRevision != fs.fencerev {
			db.DPrintf(db.FSETCD, "updateDir %v stale\n", fs.fencekey)
			return serr.NewErr(serr.TErrStale, fs.fencekey)
		}
		db.DPrintf(db.FSETCD, "updateDir %v version mismatch %v %v\n", dei.Path, v, resp.Responses[1])
		return serr.NewErr(serr.TErrVersion, dei.Path)
	}
	return nil
}

func (fs *FsEtcd) create(dei *DirEntInfo, dir *DirInfo, v sp.TQversion, new *DirEntInfo) *serr.Err {
	opts := new.Nf.LeaseOpts()
	b, err := proto.Marshal(new.Nf.EtcdFileProto)
	if err != nil {
		return serr.NewErrError(err)
	}
	d1, r := marshalDirInfo(dir)
	if r != nil {
		return r
	}
	// Update directory if new file/dir doesn't exist and directory
	// hasn't changed.
	cmp := []clientv3.Cmp{
		clientv3.Compare(clientv3.CreateRevision(fs.fencekey), "=", fs.fencerev),
		clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, new)), "=", 0),
		clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, dei)), "=", int64(v))}
	ops := []clientv3.Op{
		clientv3.OpPut(fs.path2key(fs.realm, new), string(b), opts...),
		clientv3.OpPut(fs.path2key(fs.realm, dei), string(d1))}
	ops1 := []clientv3.Op{
		clientv3.OpGet(fs.fencekey),
		clientv3.OpGet(fs.path2key(fs.realm, new)),
		clientv3.OpGet(fs.path2key(fs.realm, dei))}
	resp, err := fs.Clnt().Txn(context.TODO()).If(cmp...).Then(ops...).Else(ops1...).Commit()
	db.DPrintf(db.FSETCD, "Create new %v dei %v v %v %v err %v\n", new, dei, v, resp, err)
	if err != nil {
		return serr.NewErrError(err)
	}
	if !resp.Succeeded {
		if len(resp.Responses[0].GetResponseRange().Kvs) == 1 &&
			resp.Responses[0].GetResponseRange().Kvs[0].CreateRevision != fs.fencerev {
			db.DPrintf(db.FSETCD, "create %v stale\n", fs.fencekey)
			return serr.NewErr(serr.TErrStale, fs.fencekey)
		}
		if len(resp.Responses[1].GetResponseRange().Kvs) == 1 {
			db.DPrintf(db.FSETCD, "create %v exists %v\n", dir, new)
			return serr.NewErr(serr.TErrExists, fmt.Sprintf("path exists %v", fs.path2key(fs.realm, new)))
		}
		db.DPrintf(db.FSETCD, "create %v version mismatch %v %v\n", dei, v, resp.Responses[2])
		return serr.NewErr(serr.TErrVersion, dei.Path)
	}
	return nil
}

func (fs *FsEtcd) remove(dei *DirEntInfo, dir *DirInfo, v sp.TQversion, del *DirEntInfo) *serr.Err {
	d1, r := marshalDirInfo(dir)
	if r != nil {
		return r
	}
	cmp := []clientv3.Cmp{
		clientv3.Compare(clientv3.CreateRevision(fs.fencekey), "=", fs.fencerev),
		clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, del)), ">", 0),
		clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, dei)), "=", int64(v))}
	ops := []clientv3.Op{
		clientv3.OpDelete(fs.path2key(fs.realm, del)),
		clientv3.OpPut(fs.path2key(fs.realm, dei), string(d1))}
	ops1 := []clientv3.Op{
		clientv3.OpGet(fs.path2key(fs.realm, del))}
	resp, err := fs.Clnt().Txn(context.TODO()).
		If(cmp...).Then(ops...).Else(ops1...).Commit()
	db.DPrintf(db.FSETCD, "Remove dei %v %v %v %v err %v\n", dei, dir, del, resp, err)
	if err != nil {
		return serr.NewErrError(err)
	}
	if !resp.Succeeded {
		if len(resp.Responses[0].GetResponseRange().Kvs) != 1 {
			db.DPrintf(db.FSETCD, "remove from %v doesn't exist\n", del)
			return serr.NewErr(serr.TErrNotfound, del.Path)
		}
		return serr.NewErr(serr.TErrVersion, dei.Path)
	}
	return nil
}

// XXX retry
func (fs *FsEtcd) rename(dei *DirEntInfo, dir *DirInfo, v sp.TQversion, del, from *DirEntInfo) *serr.Err {
	d1, r := marshalDirInfo(dir)
	if r != nil {
		return r
	}
	var cmp []clientv3.Cmp
	var ops []clientv3.Op
	var ops1 []clientv3.Op
	if del != nil {
		cmp = []clientv3.Cmp{
			clientv3.Compare(clientv3.CreateRevision(fs.fencekey), "=", fs.fencerev),
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, from)), ">", 0),
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, del)), ">", 0),
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, dei)), "=", int64(v))}
		ops = []clientv3.Op{
			clientv3.OpDelete(fs.path2key(fs.realm, del)),
			clientv3.OpPut(fs.path2key(fs.realm, dei), string(d1))}
		ops1 = []clientv3.Op{
			clientv3.OpGet(fs.path2key(fs.realm, from))}
	} else {
		cmp = []clientv3.Cmp{
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, from)), ">", 0),
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, dei)), "=", int64(v))}
		ops = []clientv3.Op{
			clientv3.OpPut(fs.path2key(fs.realm, dei), string(d1))}
		ops1 = []clientv3.Op{
			clientv3.OpGet(fs.path2key(fs.realm, from))}
	}
	resp, err := fs.Clnt().Txn(context.TODO()).If(cmp...).Then(ops...).Else(ops1...).Commit()
	db.DPrintf(db.FSETCD, "Rename dei %v dir %v from %v %v err %v\n", dei, dir, from, resp, err)
	if err != nil {
		return serr.NewErrError(err)
	}
	if !resp.Succeeded {
		if len(resp.Responses[0].GetResponseRange().Kvs) != 1 {
			db.DPrintf(db.FSETCD, "rename from %v doesn't exist\n", from)
			return serr.NewErr(serr.TErrNotfound, from.Path)
		}
		return serr.NewErr(serr.TErrNotfound, dei.Path)
	}
	return nil
}

// XXX retry
func (fs *FsEtcd) renameAt(deif *DirEntInfo, dirf *DirInfo, vf sp.TQversion, deit *DirEntInfo, dirt *DirInfo, vt sp.TQversion, del, from *DirEntInfo) *serr.Err {
	bf, r := marshalDirInfo(dirf)
	if r != nil {
		return r
	}
	bt, r := marshalDirInfo(dirt)
	if r != nil {
		return r
	}
	var cmp []clientv3.Cmp
	var ops []clientv3.Op
	var ops1 []clientv3.Op
	if del != nil {
		cmp = []clientv3.Cmp{
			clientv3.Compare(clientv3.CreateRevision(fs.fencekey), "=", fs.fencerev),
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, from)), ">", 0),
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, del)), ">", 0),
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, deif)), "=", int64(vf)),
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, deit)), "=", int64(vt)),
		}
		ops = []clientv3.Op{
			clientv3.OpDelete(fs.path2key(fs.realm, del)),
			clientv3.OpPut(fs.path2key(fs.realm, deif), string(bf)),
			clientv3.OpPut(fs.path2key(fs.realm, deit), string(bt)),
		}
		ops1 = []clientv3.Op{
			clientv3.OpGet(fs.path2key(fs.realm, from))}
	} else {
		cmp = []clientv3.Cmp{
			clientv3.Compare(clientv3.CreateRevision(fs.fencekey), "=", fs.fencerev),
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, from)), ">", 0),
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, deif)), "=", int64(vf)),
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, deit)), "=", int64(vt)),
		}
		ops = []clientv3.Op{
			clientv3.OpPut(fs.path2key(fs.realm, deif), string(bf)),
			clientv3.OpPut(fs.path2key(fs.realm, deit), string(bt)),
		}
		ops1 = []clientv3.Op{
			clientv3.OpGet(fs.path2key(fs.realm, from))}
	}
	resp, err := fs.Clnt().Txn(context.TODO()).If(cmp...).Then(ops...).Else(ops1...).Commit()
	db.DPrintf(db.FSETCD, "RenameAt %v %v err %v\n", del, resp, err)
	if err != nil {
		if len(resp.Responses[0].GetResponseRange().Kvs) != 1 {
			db.DPrintf(db.FSETCD, "renameat from %v doesn't exist\n", from)
			return serr.NewErr(serr.TErrNotfound, from.Path)
		}
		return serr.NewErrError(err)
	}
	if !resp.Succeeded {
		return serr.NewErr(serr.TErrNotfound, del)
	}
	return nil
}
