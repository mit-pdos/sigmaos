package fsetcd

import (
	"context"
	"fmt"
	"strconv"

	"go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/sortedmap"
)

type Tstat int
type Tcacheable int

const (
	BOOT sp.Tpath = 0

	TSTAT_NONE Tstat = iota
	TSTAT_STAT

	EPHEMERAL = "t-"
)

type EphemeralKey struct {
	Realm sp.Trealm
	Path  sp.Tpath
	Pn    path.Tpathname
}

func (fs *FsEtcd) path2key(realm sp.Trealm, dei *DirEntInfo) string {
	return string(realm) + ":" + strconv.FormatUint(uint64(dei.Path), 16)
}

func (fs *FsEtcd) ephemkey(dei *DirEntInfo) string {
	return EPHEMERAL + string(fs.realm) + ":" + strconv.FormatUint(uint64(dei.Path), 16)
}

func (fs *FsEtcd) EphemeralPaths(realm sp.Trealm) ([]EphemeralKey, error) {
	resp, err := fs.Clnt().Get(context.TODO(), prefixEphemeral(fs.realm), clientv3.WithPrefix())
	db.DPrintf(db.FSETCD, "EphemeralPaths %v err %v\n", resp, err)
	if err != nil {
		return nil, serr.NewErrError(err)
	}
	ekeys := make([]EphemeralKey, 0)
	for _, kv := range resp.Kvs {
		r, p := key2path(string(kv.Key))
		ekeys = append(ekeys, EphemeralKey{
			Realm: r,
			Path:  p,
			Pn:    path.Split(string(kv.Value)),
		})
	}
	return ekeys, nil
}

func (fs *FsEtcd) GetEphemPathName(key string) (path.Tpathname, error) {
	resp, err := fs.Clnt().Get(context.TODO(), key)
	db.DPrintf(db.FSETCD, "GetEphemPath %v err %v\n", resp, err)
	if err != nil {
		return nil, serr.NewErrError(err)
	}
	if len(resp.Kvs) != 1 {
		rp := key2realmpath(key)
		return nil, serr.NewErr(serr.TErrNotfound, rp)
	}
	return path.Split(string(resp.Kvs[0].Value)), nil
}

func (fs *FsEtcd) getFile(key string) (*EtcdFile, sp.TQversion, *serr.Err) {
	db.DPrintf(db.FSETCD, "getFile %v\n", key)
	resp, err := fs.Clnt().Get(context.TODO(), key)
	db.DPrintf(db.FSETCD, "getFile %v %v err %v\n", key, resp, err)
	if err != nil {
		return nil, 0, serr.NewErrError(err)
	}
	if len(resp.Kvs) != 1 {
		return nil, 0, serr.NewErr(serr.TErrNotfound, key2realmpath(key))
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
	opts := dei.LeaseOpts()
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
			clientv3.OpPut(fs.path2key(fs.realm, dei), string(b)),
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
	dir, v, stat, err := fs.readDirEtcd(dei, stat)
	if err != nil {
		return nil, v, err
	}
	fs.dc.insert(dei.Path, &dcEntry{dir, v, stat})
	return dir, v, nil
}

// If stat is TSTAT_STAT, stat every entry in the directory.
func (fs *FsEtcd) readDirEtcd(dei *DirEntInfo, stat Tstat) (*DirInfo, sp.TQversion, Tstat, *serr.Err) {
	db.DPrintf(db.FSETCD, "readDirEtcd %v %v\n", dei.Path, stat)
	nf, v, err := fs.GetFile(dei)
	if err != nil {
		return nil, 0, stat, err
	}
	dir, err := UnmarshalDir(nf.Data)
	if err != nil {
		return nil, 0, stat, err
	}
	dents := sortedmap.NewSortedMap[string, *DirEntInfo]()
	update := false
	nstat := 0
	for _, e := range dir.Ents {
		if e.Name == "." {
			dents.Insert(e.Name, NewDirEntInfo(nf, e.Tpath(), e.Tperm(), e.TclntId(), e.TleaseId()))
			nstat += 1
		} else {
			di := NewDirEntInfoP(e.Tpath(), e.Tperm())
			if e.Tperm().IsEphemeral() {
				// if file is emphemeral, etcd may have expired it
				// when named didn't cache the directory, check if its
				// ephem key still exists.
				_, err := fs.GetEphemPathName(fs.ephemkey(di))
				if err != nil {
					db.DPrintf(db.FSETCD, "readDir: expired %q %v err %v\n", e.Name, e.Tperm(), err)
					update = true
					continue
				}
			}
			if stat == TSTAT_STAT {
				nf, _, err := fs.GetFile(di)
				if err != nil {
					db.DPrintf(db.ERROR, "readDir: stat entry %v %v err %v\n", e.Name, e.Tperm(), err)
					continue
				}
				nstat += 1
				di.Nf = nf
				dents.Insert(e.Name, di)
			} else {
				dents.Insert(e.Name, NewDirEntInfoP(e.Tpath(), e.Tperm()))
			}
		}
	}

	// if we stat-ed all entries, return we did so
	if nstat == dents.Len() {
		stat = TSTAT_STAT
	}

	di := &DirInfo{dents, nf.Tperm()}
	if update {
		if err := fs.updateDir(dei, di, v); err != nil {
			if err.IsErrVersion() {
				// retry?
				return di, v, stat, nil
			}
			return nil, 0, stat, err
		}
		v = v + 1
	}
	return di, v, stat, nil
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

func (fs *FsEtcd) create(dei *DirEntInfo, dir *DirInfo, v sp.TQversion, new *DirEntInfo, npn path.Tpathname) *serr.Err {
	opts := new.LeaseOpts()
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
		clientv3.OpPut(fs.path2key(fs.realm, new), string(b)),
		clientv3.OpPut(fs.path2key(fs.realm, dei), string(d1))}
	ops1 := []clientv3.Op{
		clientv3.OpGet(fs.fencekey),
		clientv3.OpGet(fs.path2key(fs.realm, new)),
		clientv3.OpGet(fs.path2key(fs.realm, dei))}

	if new.Perm.IsEphemeral() {
		ops = append(ops, clientv3.OpPut(fs.ephemkey(new), npn.String(), opts...))

	}
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
		clientv3.OpGet(fs.fencekey),
		clientv3.OpGet(fs.path2key(fs.realm, del)),
		clientv3.OpGet(fs.path2key(fs.realm, dei))}

	if del.Perm.IsEphemeral() {
		ops = append(ops, clientv3.OpDelete(fs.ephemkey(del)))
	}

	resp, err := fs.Clnt().Txn(context.TODO()).
		If(cmp...).Then(ops...).Else(ops1...).Commit()
	db.DPrintf(db.FSETCD, "Remove dei %v %v %v %v err %v\n", dei, dir, del, resp, err)
	if err != nil {
		return serr.NewErrError(err)
	}
	if !resp.Succeeded {
		if len(resp.Responses[0].GetResponseRange().Kvs) == 1 &&
			resp.Responses[0].GetResponseRange().Kvs[0].CreateRevision != fs.fencerev {
			db.DPrintf(db.FSETCD, "remove %v stale\n", fs.fencekey)
			return serr.NewErr(serr.TErrStale, fs.fencekey)
		}
		if len(resp.Responses[1].GetResponseRange().Kvs) != 1 {
			db.DPrintf(db.FSETCD, "remove from %v doesn't exist\n", del)
			return serr.NewErr(serr.TErrNotfound, del.Path)
		}
		db.DPrintf(db.FSETCD, "remove %v version mismatch %v %v\n", dei, v, resp.Responses[2])
		return serr.NewErr(serr.TErrVersion, dei.Path)
	}
	return nil
}

// XXX retry
func (fs *FsEtcd) rename(dei *DirEntInfo, dir *DirInfo, v sp.TQversion, del, from *DirEntInfo, npn path.Tpathname) *serr.Err {
	opts := from.LeaseOpts()
	d1, r := marshalDirInfo(dir)
	if r != nil {
		return r
	}
	var cmp []clientv3.Cmp
	var ops []clientv3.Op
	ops1 := []clientv3.Op{
		clientv3.OpGet(fs.fencekey),
		clientv3.OpGet(fs.path2key(fs.realm, from)),
		clientv3.OpGet(fs.path2key(fs.realm, dei))}
	if del != nil {
		cmp = []clientv3.Cmp{
			clientv3.Compare(clientv3.CreateRevision(fs.fencekey), "=", fs.fencerev),
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, from)), ">", 0),
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, del)), ">", 0),
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, dei)), "=", int64(v))}
		ops = []clientv3.Op{
			clientv3.OpDelete(fs.path2key(fs.realm, del)),
			clientv3.OpPut(fs.path2key(fs.realm, dei), string(d1))}
	} else {
		cmp = []clientv3.Cmp{
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, from)), ">", 0),
			clientv3.Compare(clientv3.Version(fs.path2key(fs.realm, dei)), "=", int64(v))}
		ops = []clientv3.Op{
			clientv3.OpPut(fs.path2key(fs.realm, dei), string(d1))}
	}

	if from.Perm.IsEphemeral() {
		ops = append(ops, clientv3.OpPut(fs.ephemkey(from), npn.String(), opts...))
	}

	resp, err := fs.Clnt().Txn(context.TODO()).If(cmp...).Then(ops...).Else(ops1...).Commit()
	db.DPrintf(db.FSETCD, "Rename dei %v dir %v from %v %v err %v\n", dei, dir, from, resp, err)
	if err != nil {
		return serr.NewErrError(err)
	}
	if !resp.Succeeded {
		if len(resp.Responses[0].GetResponseRange().Kvs) == 1 &&
			resp.Responses[0].GetResponseRange().Kvs[0].CreateRevision != fs.fencerev {
			db.DPrintf(db.FSETCD, "rename %v stale\n", fs.fencekey)
			return serr.NewErr(serr.TErrStale, fs.fencekey)
		}
		if len(resp.Responses[1].GetResponseRange().Kvs) != 1 {
			db.DPrintf(db.FSETCD, "rename from %v doesn't exist\n", from)
			return serr.NewErr(serr.TErrNotfound, from.Path)
		}
		if len(resp.Responses[2].GetResponseRange().Kvs) == 1 &&
			int64(v) != resp.Responses[2].GetResponseRange().Kvs[0].Version {
			db.DPrintf(db.FSETCD, "rename %v version mismatch %v %v\n", dei, v, resp.Responses[2])
			return serr.NewErr(serr.TErrVersion, dei.Path)
		}
		return serr.NewErr(serr.TErrNotfound, del.Path)
	}
	return nil
}

// XXX retry
func (fs *FsEtcd) renameAt(deif *DirEntInfo, dirf *DirInfo, vf sp.TQversion, deit *DirEntInfo, dirt *DirInfo, vt sp.TQversion, del, from *DirEntInfo, npn path.Tpathname) *serr.Err {
	opts := from.LeaseOpts()
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
	ops1 := []clientv3.Op{
		clientv3.OpGet(fs.fencekey),
		clientv3.OpGet(fs.path2key(fs.realm, from)),
		clientv3.OpGet(fs.path2key(fs.realm, deif)),
		clientv3.OpGet(fs.path2key(fs.realm, deit))}
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
	}

	if from.Perm.IsEphemeral() {
		ops = append(ops, clientv3.OpPut(fs.ephemkey(from), npn.String(), opts...))
	}

	resp, err := fs.Clnt().Txn(context.TODO()).If(cmp...).Then(ops...).Else(ops1...).Commit()
	db.DPrintf(db.FSETCD, "RenameAt %v %v err %v\n", del, resp, err)
	if err != nil {
		return serr.NewErrError(err)
	}
	if !resp.Succeeded {
		if len(resp.Responses[0].GetResponseRange().Kvs) == 1 &&
			resp.Responses[0].GetResponseRange().Kvs[0].CreateRevision != fs.fencerev {
			db.DPrintf(db.FSETCD, "renameat %v stale\n", fs.fencekey)
			return serr.NewErr(serr.TErrStale, fs.fencekey)
		}
		if len(resp.Responses[1].GetResponseRange().Kvs) != 1 {
			db.DPrintf(db.FSETCD, "renameat from %v doesn't exist\n", from)
			return serr.NewErr(serr.TErrNotfound, from.Path)
		}
		if len(resp.Responses[2].GetResponseRange().Kvs) == 1 &&
			int64(vf) != resp.Responses[2].GetResponseRange().Kvs[0].Version {
			db.DPrintf(db.FSETCD, "renameat %v version mismatch %v %v\n", deif, vf, resp.Responses[2])
			return serr.NewErr(serr.TErrVersion, deif.Path)
		}
		if len(resp.Responses[3].GetResponseRange().Kvs) == 1 &&
			int64(vf) != resp.Responses[3].GetResponseRange().Kvs[0].Version {
			db.DPrintf(db.FSETCD, "renameat %v version mismatch %v %v\n", deit, vt, resp.Responses[3])
			return serr.NewErr(serr.TErrVersion, deit.Path)
		}
		return serr.NewErr(serr.TErrNotfound, del.Path)
	}
	return nil
}
