package fsetcd

import (
	"context"
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

	TCACHEABLE_NO Tcacheable = iota
	TCACHEABLE_YES
)

func (fs *FsEtcd) path2key(realm sp.Trealm, path sp.Tpath) string {
	return string(realm) + ":" + strconv.FormatUint(uint64(path), 16)
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
	nf := &EtcdFile{}
	if err := proto.Unmarshal(resp.Kvs[0].Value, nf); err != nil {
		return nil, 0, serr.NewErrError(err)
	}
	db.DPrintf(db.FSETCD, "getFile %v %v\n", key, nf)
	return nf, sp.TQversion(resp.Kvs[0].Version), nil
}

func (fs *FsEtcd) GetFile(p sp.Tpath) (*EtcdFile, sp.TQversion, *serr.Err) {
	return fs.getFile(fs.path2key(fs.realm, p))
}

func (fs *FsEtcd) PutFile(p sp.Tpath, nf *EtcdFile, f sp.Tfence) *serr.Err {
	opts := nf.LeaseOpts()
	if b, err := proto.Marshal(nf); err != nil {
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
			clientv3.OpPut(fs.path2key(fs.realm, p), string(b), opts...),
		}
		opsf := []clientv3.Op{
			clientv3.OpGet(f.Prefix(), opts...),
		}
		resp, err := fs.Clnt().Txn(context.TODO()).If(cmp...).Then(opst...).Else(opsf...).Commit()
		db.DPrintf(db.FSETCD, "PutFile %v %v %v %v err %v\n", p, nf, f, resp, err)
		if err != nil {
			return serr.NewErrError(err)
		}
		if !resp.Succeeded {
			if len(resp.Responses[0].GetResponseRange().Kvs) != 1 {
				db.DPrintf(db.FSETCD, "PutFile %v %v %v %v stale\n", p, nf, f, resp)
				return serr.NewErr(serr.TErrStale, f)
			}
			db.DFatalf("PutFile failed %v %v %v\n", p, nf, resp.Responses[0])
		}

		return nil
	}
}

func (fs *FsEtcd) readDir(p sp.Tpath, stat Tstat) (*DirInfo, sp.TQversion, *serr.Err) {
	if dir, v, st, ok := fs.dc.lookup(p); ok && (stat == TSTAT_NONE || st == TSTAT_STAT) {
		db.DPrintf(db.FSETCD, "fsetcd.readDir %v\n", dir)
		return dir, v, nil
	}
	dir, v, c, err := fs.readDirEtcd(p, stat)
	if err != nil {
		return nil, v, err
	}
	if c == TCACHEABLE_YES {
		db.DPrintf(db.FSETCD, "fsetcd.readDir cacheable %v %v\n", p, dir)
		fs.dc.insert(p, dir, v, stat)
	}
	return dir, v, nil
}

// If stat is TSTAT_STAT, stat every entry in the directory.  If entry
// is ephemeral, stat entry and filter it out if expired.  If
// directory contains ephemeral entries, return TCACHEABLE_NO.
func (fs *FsEtcd) readDirEtcd(p sp.Tpath, stat Tstat) (*DirInfo, sp.TQversion, Tcacheable, *serr.Err) {
	db.DPrintf(db.FSETCD, "readDirEtcd %v\n", p)
	nf, v, err := fs.GetFile(p)
	if err != nil {
		return nil, 0, TCACHEABLE_NO, err
	}
	dir, err := UnmarshalDir(nf.Data)
	if err != nil {
		return nil, 0, TCACHEABLE_NO, err
	}
	dents := sorteddir.NewSortedDir()
	cacheable := TCACHEABLE_YES
	update := false
	for _, e := range dir.Ents {
		if e.Name == "." {
			dents.Insert(e.Name, DirEntInfo{nf, e.Tpath(), e.Tperm()})
		} else {
			if e.Tperm().IsEphemeral() || stat == TSTAT_STAT {
				// if file is emphemeral, etcd may have expired it, so
				// check if it still exists; if not, don't return the
				// entry.
				nf, _, err := fs.GetFile(e.Tpath())
				if err != nil {
					db.DPrintf(db.FSETCD, "readDir: expired %v err %v\n", e.Name, err)
					update = true
					continue
				}
				if e.Tperm().IsEphemeral() {
					db.DPrintf(db.FSETCD, "readDir: %v ephemeral; not cacheable %v\n", e.Name, e.Tperm())
					cacheable = TCACHEABLE_NO
				}
				dents.Insert(e.Name, DirEntInfo{nf, e.Tpath(), e.Tperm()})
			} else {
				dents.Insert(e.Name, DirEntInfo{nil, e.Tpath(), e.Tperm()})
			}
		}
	}
	di := &DirInfo{dents, nf.Tperm()}
	if update {
		if err := fs.updateDir(p, di, v); err != nil {
			return nil, 0, TCACHEABLE_NO, err
		}
		v = v + 1
	}
	return di, v, cacheable, nil
}

func (fs *FsEtcd) updateDir(dp sp.Tpath, dir *DirInfo, v sp.TQversion) *serr.Err {
	d1, r := marshalDirInfo(dir)
	if r != nil {
		return r
	}
	// Update directory if directory hasn't changed.
	cmp := []clientv3.Cmp{
		clientv3.Compare(clientv3.CreateRevision(fs.fencekey), "=", fs.fencerev),
		clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, dp)), "=", int64(v))}
	ops := []clientv3.Op{
		clientv3.OpPut(fs.path2key(fs.realm, dp), string(d1))}
	resp, err := fs.Clnt().Txn(context.TODO()).If(cmp...).Then(ops...).Commit()
	db.DPrintf(db.FSETCD, "updateDir %v %v %v %v err %v\n", dp, dir, v, resp, err)
	if err != nil {
		return serr.NewErrError(err)
	}
	if !resp.Succeeded {
		db.DPrintf(db.FSETCD, "updateDir %v %v %v %v stale\n", dp, dir, v, resp)
		return serr.NewErr(serr.TErrStale, dp)
	}
	return nil
}

func (fs *FsEtcd) create(dp sp.Tpath, dir *DirInfo, v sp.TQversion, p sp.Tpath, nf *EtcdFile) *serr.Err {
	opts := nf.LeaseOpts()
	b, err := proto.Marshal(nf)
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
		clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, p)), "=", 0),
		clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, dp)), "=", int64(v))}
	ops := []clientv3.Op{
		clientv3.OpPut(fs.path2key(fs.realm, p), string(b), opts...),
		clientv3.OpPut(fs.path2key(fs.realm, dp), string(d1))}
	resp, err := fs.Clnt().Txn(context.TODO()).If(cmp...).Then(ops...).Commit()
	db.DPrintf(db.FSETCD, "Create %v %v %v with lease %x err %v\n", p, dir, resp, nf.LeaseId, err)
	if err != nil {
		return serr.NewErrError(err)
	}
	if !resp.Succeeded {
		return serr.NewErr(serr.TErrExists, p)
	}
	return nil
}

func (fs *FsEtcd) remove(d sp.Tpath, dir *DirInfo, v sp.TQversion, del sp.Tpath) *serr.Err {
	d1, r := marshalDirInfo(dir)
	if r != nil {
		return r
	}
	cmp := []clientv3.Cmp{
		clientv3.Compare(clientv3.CreateRevision(fs.fencekey), "=", fs.fencerev),
		clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, del)), ">", 0),
		clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, d)), "=", int64(v))}
	ops := []clientv3.Op{
		clientv3.OpDelete(fs.path2key(fs.realm, del)),
		clientv3.OpPut(fs.path2key(fs.realm, d), string(d1))}
	resp, err := fs.Clnt().Txn(context.TODO()).
		If(cmp...).Then(ops...).Commit()
	db.DPrintf(db.FSETCD, "Remove %v %v %v %v err %v\n", d, dir, del, resp, err)
	if err != nil {
		return serr.NewErrError(err)
	}
	if !resp.Succeeded {
		return serr.NewErr(serr.TErrNotfound, del)
	}
	return nil
}

// XXX retry
func (fs *FsEtcd) rename(d sp.Tpath, dir *DirInfo, v sp.TQversion, del sp.Tpath) *serr.Err {
	d1, r := marshalDirInfo(dir)
	if r != nil {
		return r
	}
	var cmp []clientv3.Cmp
	var ops []clientv3.Op
	if del != 0 {
		cmp = []clientv3.Cmp{
			clientv3.Compare(clientv3.CreateRevision(fs.fencekey), "=", fs.fencerev),
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, del)), ">", 0),
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, d)), "=", int64(v))}
		ops = []clientv3.Op{
			clientv3.OpDelete(fs.path2key(fs.realm, del)),
			clientv3.OpPut(fs.path2key(fs.realm, d), string(d1))}
	} else {
		cmp = []clientv3.Cmp{
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, d)), "=", int64(v))}
		ops = []clientv3.Op{
			clientv3.OpPut(fs.path2key(fs.realm, d), string(d1))}
	}
	resp, err := fs.Clnt().Txn(context.TODO()).If(cmp...).Then(ops...).Commit()
	db.DPrintf(db.FSETCD, "Rename %v %v %v err %v\n", d, dir, resp, err)
	if err != nil {
		return serr.NewErrError(err)
	}
	if !resp.Succeeded {
		return serr.NewErr(serr.TErrNotfound, d)
	}
	return nil
}

// XXX retry
func (fs *FsEtcd) renameAt(df sp.Tpath, dirf *DirInfo, vf sp.TQversion, dt sp.Tpath, dirt *DirInfo, vt sp.TQversion, del sp.Tpath) *serr.Err {
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
	if del != 0 {
		cmp = []clientv3.Cmp{
			clientv3.Compare(clientv3.CreateRevision(fs.fencekey), "=", fs.fencerev),
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, del)), ">", 0),
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, df)), "=", int64(vf)),
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, dt)), "=", int64(vt)),
		}
		ops = []clientv3.Op{
			clientv3.OpDelete(fs.path2key(fs.realm, del)),
			clientv3.OpPut(fs.path2key(fs.realm, df), string(bf)),
			clientv3.OpPut(fs.path2key(fs.realm, dt), string(bt)),
		}
	} else {
		cmp = []clientv3.Cmp{
			clientv3.Compare(clientv3.CreateRevision(fs.fencekey), "=", fs.fencerev),
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, df)), "=", int64(vf)),
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, dt)), "=", int64(vt)),
		}
		ops = []clientv3.Op{
			clientv3.OpPut(fs.path2key(fs.realm, df), string(bf)),
			clientv3.OpPut(fs.path2key(fs.realm, dt), string(bt)),
		}
	}
	resp, err := fs.Clnt().Txn(context.TODO()).If(cmp...).Then(ops...).Commit()
	db.DPrintf(db.FSETCD, "RenameAt %v %v err %v\n", del, resp, err)
	if err != nil {
		return serr.NewErrError(err)
	}
	if !resp.Succeeded {
		return serr.NewErr(serr.TErrNotfound, del)
	}
	return nil
}
