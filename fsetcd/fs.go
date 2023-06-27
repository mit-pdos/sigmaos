package fsetcd

import (
	"context"
	"strconv"

	"go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/sorteddir"
)

const (
	BOOT sessp.Tpath = 0
)

func (ec *EtcdClnt) path2key(path sessp.Tpath) string {
	return string(ec.realm) + ":" + strconv.FormatUint(uint64(path), 16)
}

func (ec *EtcdClnt) getFile(key string) (*EtcdFile, sp.TQversion, *serr.Err) {
	resp, err := ec.Get(context.TODO(), key)
	if err != nil {
		return nil, 0, serr.MkErrError(err)
	}
	db.DPrintf(db.ETCDCLNT, "GetFile %v %v\n", key, resp)
	if len(resp.Kvs) != 1 {
		return nil, 0, serr.MkErr(serr.TErrNotfound, key2path(key))
	}
	nf := &EtcdFile{}
	if err := proto.Unmarshal(resp.Kvs[0].Value, nf); err != nil {
		return nil, 0, serr.MkErrError(err)
	}
	db.DPrintf(db.ETCDCLNT, "GetFile %v %v\n", key, nf)
	return nf, sp.TQversion(resp.Kvs[0].Version), nil
}

func (ec *EtcdClnt) GetFile(p sessp.Tpath) (*EtcdFile, sp.TQversion, *serr.Err) {
	return ec.getFile(ec.path2key(p))
}

func (ec *EtcdClnt) PutFile(p sessp.Tpath, nf *EtcdFile) *serr.Err {
	opts, sr := ec.lmgr.LeaseOpts(nf)
	if sr != nil {
		return sr
	}
	if b, err := proto.Marshal(nf); err != nil {
		return serr.MkErrError(err)
	} else {
		cmp := []clientv3.Cmp{
			clientv3.Compare(clientv3.CreateRevision(ec.fencekey), "=", ec.fencerev),
		}
		ops := []clientv3.Op{
			clientv3.OpPut(ec.path2key(p), string(b), opts...),
		}
		resp, err := ec.Txn(context.TODO()).If(cmp...).Then(ops...).Commit()
		if err != nil {
			return serr.MkErrError(err)
		}

		db.DPrintf(db.ETCDCLNT, "PutFile %v %v %v\n", p, nf, resp)
		return nil
	}
}

func (ec *EtcdClnt) readDir(p sessp.Tpath, stat bool) (*DirInfo, sp.TQversion, *serr.Err) {
	db.DPrintf(db.ETCDCLNT, "readDir %v\n", p)
	nf, v, err := ec.GetFile(p)
	if err != nil {
		return nil, 0, err
	}
	dir, err := UnmarshalDir(nf.Data)
	if err != nil {
		return nil, 0, err
	}
	dents := sorteddir.MkSortedDir()
	for _, e := range dir.Ents {
		if e.Name == "." {
			dents.Insert(e.Name, DirEntInfo{nf, e.Tpath(), e.Tperm()})
		} else {
			if e.Tperm().IsEphemeral() || true {
				// if file is emphemeral, etcd may have expired it, so
				// check if it still exists; if not, don't return the
				// entry.
				nf, _, err := ec.GetFile(e.Tpath())
				if err != nil {
					db.DPrintf(db.ETCDCLNT, "readDir: GetFile %v %v\n", e.Name, err)
					continue
				}
				dents.Insert(e.Name, DirEntInfo{nf, e.Tpath(), e.Tperm()})
			} else {
				dents.Insert(e.Name, DirEntInfo{nil, e.Tpath(), e.Tperm()})
			}
		}
	}
	return &DirInfo{dents, nf.Tperm()}, v, nil
}

func (ec *EtcdClnt) create(dp sessp.Tpath, dir *DirInfo, v sp.TQversion, p sessp.Tpath, nf *EtcdFile) *serr.Err {
	opts, sr := ec.lmgr.LeaseOpts(nf)
	if sr != nil {
		return sr
	}
	b, err := proto.Marshal(nf)
	if err != nil {
		return serr.MkErrError(err)
	}
	d1, r := marshalDirInfo(dir)
	if r != nil {
		return r
	}
	// Update directory if new file/dir doesn't exist and directory
	// hasn't changed.
	cmp := []clientv3.Cmp{
		clientv3.Compare(clientv3.CreateRevision(ec.fencekey), "=", ec.fencerev),
		clientv3.Compare(clientv3.Version(ec.path2key(p)), "=", 0),
		clientv3.Compare(clientv3.Version(ec.path2key(dp)), "=", int64(v))}
	ops := []clientv3.Op{
		clientv3.OpPut(ec.path2key(p), string(b), opts...),
		clientv3.OpPut(ec.path2key(dp), string(d1))}
	resp, err := ec.Txn(context.TODO()).If(cmp...).Then(ops...).Commit()
	if err != nil {
		return serr.MkErrError(err)
	}
	db.DPrintf(db.ETCDCLNT, "Create %v %v with lease %x\n", p, resp, nf.LeaseId)
	if !resp.Succeeded {
		return serr.MkErr(serr.TErrExists, p)
	}
	return nil
}

func (ec *EtcdClnt) remove(d sessp.Tpath, dir *DirInfo, v sp.TQversion, del sessp.Tpath) *serr.Err {
	d1, r := marshalDirInfo(dir)
	if r != nil {
		return r
	}
	cmp := []clientv3.Cmp{
		clientv3.Compare(clientv3.CreateRevision(ec.fencekey), "=", ec.fencerev),
		clientv3.Compare(clientv3.Version(ec.path2key(del)), ">", 0),
		clientv3.Compare(clientv3.Version(ec.path2key(d)), "=", int64(v))}
	ops := []clientv3.Op{
		clientv3.OpDelete(ec.path2key(del)),
		clientv3.OpPut(ec.path2key(d), string(d1))}
	resp, err := ec.Txn(context.TODO()).
		If(cmp...).Then(ops...).Commit()
	if err != nil {
		return serr.MkErrError(err)
	}
	db.DPrintf(db.ETCDCLNT, "Remove %v %v\n", del, resp)
	if !resp.Succeeded {
		return serr.MkErr(serr.TErrNotfound, del)
	}
	return nil
}

// XXX retry
func (ec *EtcdClnt) rename(d sessp.Tpath, dir *DirInfo, v sp.TQversion, del sessp.Tpath) *serr.Err {
	d1, r := marshalDirInfo(dir)
	if r != nil {
		return r
	}
	var cmp []clientv3.Cmp
	var ops []clientv3.Op
	if del != 0 {
		cmp = []clientv3.Cmp{
			clientv3.Compare(clientv3.CreateRevision(ec.fencekey), "=", ec.fencerev),
			clientv3.Compare(clientv3.Version(ec.path2key(del)), ">", 0),
			clientv3.Compare(clientv3.Version(ec.path2key(d)), "=", int64(v))}
		ops = []clientv3.Op{
			clientv3.OpDelete(ec.path2key(del)),
			clientv3.OpPut(ec.path2key(d), string(d1))}
	} else {
		cmp = []clientv3.Cmp{
			clientv3.Compare(clientv3.Version(ec.path2key(d)), "=", int64(v))}
		ops = []clientv3.Op{
			clientv3.OpPut(ec.path2key(d), string(d1))}
	}
	resp, err := ec.Txn(context.TODO()).If(cmp...).Then(ops...).Commit()
	if err != nil {
		return serr.MkErrError(err)
	}
	db.DPrintf(db.ETCDCLNT, "Rename %v %v\n", d, resp)
	if !resp.Succeeded {
		return serr.MkErr(serr.TErrNotfound, d)
	}
	return nil
}

// XXX retry
func (ec *EtcdClnt) renameAt(df sessp.Tpath, dirf *DirInfo, vf sp.TQversion, dt sessp.Tpath, dirt *DirInfo, vt sp.TQversion, del sessp.Tpath) *serr.Err {
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
			clientv3.Compare(clientv3.CreateRevision(ec.fencekey), "=", ec.fencerev),
			clientv3.Compare(clientv3.Version(ec.path2key(del)), ">", 0),
			clientv3.Compare(clientv3.Version(ec.path2key(df)), "=", int64(vf)),
			clientv3.Compare(clientv3.Version(ec.path2key(dt)), "=", int64(vt)),
		}
		ops = []clientv3.Op{
			clientv3.OpDelete(ec.path2key(del)),
			clientv3.OpPut(ec.path2key(df), string(bf)),
			clientv3.OpPut(ec.path2key(dt), string(bt)),
		}
	} else {
		cmp = []clientv3.Cmp{
			clientv3.Compare(clientv3.CreateRevision(ec.fencekey), "=", ec.fencerev),
			clientv3.Compare(clientv3.Version(ec.path2key(df)), "=", int64(vf)),
			clientv3.Compare(clientv3.Version(ec.path2key(dt)), "=", int64(vt)),
		}
		ops = []clientv3.Op{
			clientv3.OpPut(ec.path2key(df), string(bf)),
			clientv3.OpPut(ec.path2key(dt), string(bt)),
		}
	}
	resp, err := ec.Txn(context.TODO()).If(cmp...).Then(ops...).Commit()
	if err != nil {
		return serr.MkErrError(err)
	}
	db.DPrintf(db.ETCDCLNT, "RenameAt %v %v\n", del, resp)
	if !resp.Succeeded {
		return serr.MkErr(serr.TErrNotfound, del)
	}
	return nil
}
