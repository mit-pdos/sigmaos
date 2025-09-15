package fsetcd

import (
	"context"
	"fmt"
	"strconv"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/util/sortedmapv1"
	"sigmaos/util/spstats"
)

type Tstat int
type Tcacheable int

const (
	BOOT sp.Tpath = 0

	TSTAT_NONE Tstat = iota
	TSTAT_STAT

	LEASEPREFIX = "_l-"
)

func (st Tstat) String() string {
	switch st {
	case TSTAT_NONE:
		return "stat_none"
	case TSTAT_STAT:
		return "stat_stat"
	default:
		db.DFatalf("Unknown stat: %v", int(st))
		return "unknown-stat"
	}
}

type LeasedKey struct {
	Realm sp.Trealm
	Path  sp.Tpath
	Pn    path.Tpathname
}

func (fs *FsEtcd) path2key(realm sp.Trealm, dei *DirEntInfo) string {
	return string(realm) + ":" + strconv.FormatUint(uint64(dei.Path), 16)
}

func (fs *FsEtcd) leasedkey(dei *DirEntInfo) string {
	return LEASEPREFIX + string(fs.realm) + ":" + strconv.FormatUint(uint64(dei.Path), 16)
}

func (fs *FsEtcd) LeasedPaths(realm sp.Trealm) ([]LeasedKey, spstats.Tcounter, error) {
	c := spstats.NewCounter(1)
	resp, err := fs.Clnt().Get(context.TODO(), prefixLease(fs.realm), clientv3.WithPrefix())
	db.DPrintf(db.FSETCD, "LeasedPaths %v err %v\n", resp, err)
	if err != nil {
		return nil, c, serr.NewErrError(err)
	}
	ekeys := make([]LeasedKey, 0)
	for _, kv := range resp.Kvs {
		r, p := key2path(string(kv.Key))
		ekeys = append(ekeys, LeasedKey{
			Realm: r,
			Path:  p,
			Pn:    path.Split(string(kv.Value)),
		})
	}
	return ekeys, c, nil
}

func (fs *FsEtcd) getLeasedPathName(key string) (path.Tpathname, spstats.Tcounter, error) {
	c := spstats.NewCounter(1)
	resp, err := fs.Clnt().Get(context.TODO(), key)
	db.DPrintf(db.FSETCD, "getLeasedPathName %v err %v\n", resp, err)
	if err != nil {
		return nil, c, serr.NewErrError(err)
	}
	if len(resp.Kvs) != 1 {
		rp := key2realmpath(key)
		return nil, c, serr.NewErr(serr.TErrNotfound, rp)
	}
	return path.Split(string(resp.Kvs[0].Value)), c, nil
}

func (fs *FsEtcd) getFile(key string) (*EtcdFile, sp.TQversion, spstats.Tcounter, *serr.Err) {
	c := spstats.NewCounter(1)
	db.DPrintf(db.FSETCD, "getFile %v\n", key)
	resp, err := fs.Clnt().Get(context.TODO(), key)
	db.DPrintf(db.FSETCD, "getFile %v %v err %v\n", key, resp, err)
	if err != nil {
		return nil, 0, c, serr.NewErrError(err)
	}
	if len(resp.Kvs) != 1 {
		db.DPrintf(db.FSETCD, "getFile unexpected len(resp.Kvs) %v %v %v err %v\n", len(resp.Kvs), key, resp, err)
		return nil, 0, c, serr.NewErr(serr.TErrNotfound, key2realmpath(key))
	}
	nf := newEtcdFile()
	if err := proto.Unmarshal(resp.Kvs[0].Value, nf.EtcdFileProto); err != nil {
		return nil, 0, c, serr.NewErrError(err)
	}
	db.DPrintf(db.FSETCD, "getFile %v %v\n", key, nf)
	return nf, sp.TQversion(resp.Kvs[0].Version), c, nil
}

func (fs *FsEtcd) GetFile(dei *DirEntInfo) (*EtcdFile, sp.TQversion, spstats.Tcounter, *serr.Err) {
	return fs.getFile(fs.path2key(fs.realm, dei))
}

func (fs *FsEtcd) PutFile(dei *DirEntInfo, nf *EtcdFile, f sp.Tfence) (spstats.Tcounter, *serr.Err) {
	c := spstats.NewCounter(1)
	opts := dei.LeaseOpts()
	fenced := f.PathName != ""
	if b, err := proto.Marshal(nf.EtcdFileProto); err != nil {
		return c, serr.NewErrError(err)
	} else {
		cmp := []clientv3.Cmp{
			clientv3.Compare(clientv3.CreateRevision(fs.fencekey), "=", fs.fencerev),
		}
		opst := []clientv3.Op{
			clientv3.OpPut(fs.path2key(fs.realm, dei), string(b), opts...),
		}
		opsf := []clientv3.Op{
			clientv3.OpGet(fs.fencekey),
		}
		if fenced {
			cmp = append(cmp, clientv3.Compare(clientv3.CreateRevision(f.PathName), "=", int64(f.Epoch)))
			opsf = append(opsf, clientv3.OpGet(f.PathName))
		}
		resp, err := fs.Clnt().Txn(context.TODO()).If(cmp...).Then(opst...).Else(opsf...).Commit()
		db.DPrintf(db.FSETCD, "PutFile dei %v f %v resp %v err %v\n", dei, f, resp, err)
		if err != nil {
			return c, serr.NewErrError(err)
		}
		if !resp.Succeeded {
			if len(resp.Responses[0].GetResponseRange().Kvs) != 1 {
				db.DPrintf(db.FENCEFS, "PutFile dei %v f %v resp %v stale\n", dei, f, resp)
				return c, serr.NewErr(serr.TErrUnreachable, fs.fencekey)
			}
			if fenced && len(resp.Responses[1].GetResponseRange().Kvs) != 1 {
				db.DPrintf(db.FENCEFS, "PutFile dei %v f %v resp %v stale\n", dei, f, resp)
				return c, serr.NewErr(serr.TErrStale, f)
			}
			db.DPrintf(db.ERROR, "PutFile failed dei %v %v\n", dei, resp.Responses[0])
		}
		return c, nil
	}
}

func (fs *FsEtcd) readDir(dei *DirEntInfo, stat Tstat) (*DirInfo, sp.TQversion, spstats.Tcounter, *serr.Err) {
	if de, ok := fs.dc.lookup(dei.Path); ok && (stat == TSTAT_NONE || de.stat == TSTAT_STAT) {
		db.DPrintf(db.FSETCD, "fsetcd.readDir path %v %v", dei.Path, de.dir)
		return de.dir, de.v, spstats.NewCounter(0), nil
	}
	s := time.Now()
	dir, v, stat, nops, err := fs.readDirEtcd(dei, stat)
	if err != nil {
		return nil, v, nops, err
	}
	db.DPrintf(db.FSETCD_LAT, "readDirEtcd %v lat %v", dei.Path, time.Since(s))
	fs.dc.insert(dei.Path, newDCEntry(dir, v, stat))
	return dir, v, nops, nil
}

// If stat is TSTAT_STAT, stat every entry in the directory.
func (fs *FsEtcd) readDirEtcd(dei *DirEntInfo, stat Tstat) (*DirInfo, sp.TQversion, Tstat, spstats.Tcounter, *serr.Err) {
	db.DPrintf(db.FSETCD, "readDirEtcd %v %v\n", dei.Path, stat)
	nops := spstats.NewCounter(1)
	nf, v, nops, err := fs.GetFile(dei)
	if err != nil {
		return nil, 0, stat, nops, err
	}
	dir, err := UnmarshalDir(nf.Data)
	if err != nil {
		return nil, 0, stat, nops, err
	}
	dents := sortedmapv1.NewSortedMap[string, *DirEntInfo]()
	update := false
	nstat := 0
	for _, e := range dir.Ents {
		if e.Name == "." {
			dents.Insert(e.Name, NewDirEntInfoNf(nf, e.Tpath(), e.Tperm(), e.TclntId(), e.TleaseId()))
			nstat += 1
		} else {
			di := NewDirEntInfo(e.Tpath(), e.Tperm(), e.TclntId(), e.TleaseId())
			if di.LeaseId.IsLeased() {
				s := time.Now()
				// if file is leased, etcd may have expired it when
				// named didn't cache the directory, check if its
				// leased key still exists.
				_, nops1, err := fs.getLeasedPathName(fs.leasedkey(di))
				spstats.Add(&nops, nops1)
				if err != nil {
					db.DPrintf(db.FSETCD, "readDir: expired %q %v err %v\n", e.Name, e.Tperm(), err)
					update = true
					continue
				}
				db.DPrintf(db.FSETCD_LAT, "%v: check lease %v %v", dei.Path, e.Tpath(), time.Since(s))
			}
			if stat == TSTAT_STAT {
				s := time.Now()
				nf, _, nops1, err := fs.GetFile(di)
				spstats.Add(&nops, nops1)
				db.DPrintf(db.FSETCD_LAT, "%v: check stat %v %v", dei.Path, e.Tpath(), time.Since(s))
				if err != nil {
					db.DPrintf(db.ERROR, "readDir: stat entry %v %v err %v\n", e.Name, e.Tperm(), err)
					continue
				}
				nstat += 1
				di.Nf = nf
			}
			dents.Insert(e.Name, di)
		}
	}

	// if we stat-ed all entries, return we did so
	if nstat == dents.Len() {
		stat = TSTAT_STAT
	}

	di := &DirInfo{
		Ents: dents,
	}
	if update {
		nops1, err := fs.updateDir(dei, di, v)
		spstats.Add(&nops, nops1)
		if err != nil {
			if err.IsErrVersion() {
				// retry?
				return di, v, stat, nops, nil
			}
			return nil, 0, stat, nops, err
		}
		v = v + 1
	}
	return di, v, stat, nops, nil
}

func (fs *FsEtcd) updateDir(dei *DirEntInfo, dir *DirInfo, v sp.TQversion) (spstats.Tcounter, *serr.Err) {
	d1, r := marshalDirInfo(dir)
	if r != nil {
		return spstats.NewCounter(0), r
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
	c := spstats.NewCounter(1)
	resp, err := fs.Clnt().Txn(context.TODO()).If(cmp...).Then(ops...).Else(ops1...).Commit()
	db.DPrintf(db.FSETCD, "updateDir %v %v %v %v err %v\n", dei.Path, dir, v, resp, err)
	if err != nil {
		return c, serr.NewErrError(err)
	}
	if !resp.Succeeded {
		if len(resp.Responses[0].GetResponseRange().Kvs) != 1 {
			db.DPrintf(db.FSETCD, "updateDir %v stale\n", fs.fencekey)
			return c, serr.NewErr(serr.TErrUnreachable, fs.fencekey)
		}
		db.DPrintf(db.FSETCD, "updateDir %v version mismatch %v %v\n", dei.Path, v, resp.Responses[1])
		return c, serr.NewErr(serr.TErrVersion, dei.Path)
	}
	return c, nil
}

func (fs *FsEtcd) create(dei *DirEntInfo, dir *DirInfo, v sp.TQversion, new *DirEntInfo, npn path.Tpathname, f sp.Tfence) (spstats.Tcounter, *serr.Err) {
	c := spstats.NewCounter(0)
	opts := new.LeaseOpts()
	b, err := proto.Marshal(new.Nf.EtcdFileProto)
	if err != nil {
		return c, serr.NewErrError(err)
	}
	d1, r := marshalDirInfo(dir)
	if r != nil {
		return c, r
	}
	fenced := f.PathName != ""
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

	if fenced {
		cmp = append(cmp, clientv3.Compare(clientv3.CreateRevision(f.PathName), "=", int64(f.Epoch)))
		ops1 = append(ops1, clientv3.OpGet(f.PathName))
	}

	if new.LeaseId.IsLeased() {
		ops = append(ops, clientv3.OpPut(fs.leasedkey(new), npn.String(), opts...))

	}
	spstats.Inc(&c, 1)
	start := time.Now()
	resp, err := fs.Clnt().Txn(context.TODO()).If(cmp...).Then(ops...).Else(ops1...).Commit()
	db.DPrintf(db.FSETCD, "Create new %v dei %v v %v %v err %v lat %v", new, dei, v, resp, err, time.Since(start))
	if err != nil {
		return c, serr.NewErrError(err)
	}
	if !resp.Succeeded {
		if len(resp.Responses[0].GetResponseRange().Kvs) != 1 {
			db.DPrintf(db.FSETCD, "create %v stale", fs.fencekey)
			return c, serr.NewErr(serr.TErrUnreachable, fs.fencekey)
		}
		if fenced && len(resp.Responses[3].GetResponseRange().Kvs) != 1 {
			db.DPrintf(db.FENCEFS, "rename %v f stale", f)
			return c, serr.NewErr(serr.TErrStale, f.PathName)
		}
		if len(resp.Responses[1].GetResponseRange().Kvs) == 1 {
			db.DPrintf(db.FSETCD, "create %v exists %v", dir, new)
			return c, serr.NewErr(serr.TErrExists, fmt.Sprintf("path exists %v", fs.path2key(fs.realm, new)))
		}
		db.DPrintf(db.FSETCD, "create %v version mismatch %v %v", dei, v, resp.Responses[2])
		return c, serr.NewErr(serr.TErrVersion, dei.Path)
	}
	return c, nil
}

func (fs *FsEtcd) remove(dei *DirEntInfo, dir *DirInfo, v sp.TQversion, del *DirEntInfo, f sp.Tfence) (spstats.Tcounter, *serr.Err) {
	c := spstats.NewCounter(0)
	d1, r := marshalDirInfo(dir)
	if r != nil {
		return c, r
	}
	fenced := f.PathName != ""
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
	if fenced {
		cmp = append(cmp, clientv3.Compare(clientv3.CreateRevision(f.PathName), "=", int64(f.Epoch)))
		ops1 = append(ops1, clientv3.OpGet(f.PathName))
	}
	if del.LeaseId.IsLeased() {
		ops = append(ops, clientv3.OpDelete(fs.leasedkey(del)))
	}
	spstats.Inc(&c, 1)
	resp, err := fs.Clnt().Txn(context.TODO()).
		If(cmp...).Then(ops...).Else(ops1...).Commit()
	db.DPrintf(db.FSETCD, "Remove dei %v %v %v %v err %v\n", dei, dir, del, resp, err)
	if err != nil {
		return c, serr.NewErrError(err)
	}
	if !resp.Succeeded {
		if len(resp.Responses[0].GetResponseRange().Kvs) != 1 {
			db.DPrintf(db.FSETCD, "remove %v stale\n", fs.fencekey)
			return c, serr.NewErr(serr.TErrUnreachable, fs.fencekey)
		}
		if fenced && len(resp.Responses[3].GetResponseRange().Kvs) != 1 {
			db.DPrintf(db.FSETCD, "remove %v stale\n", f)
			return c, serr.NewErr(serr.TErrStale, f.PathName)
		}
		if len(resp.Responses[1].GetResponseRange().Kvs) != 1 {
			db.DPrintf(db.FSETCD, "remove from %v doesn't exist\n", del)
			return c, serr.NewErr(serr.TErrNotfound, del.Path)
		}
		db.DPrintf(db.FSETCD, "remove %v version mismatch %v %v\n", dei, v, resp.Responses[2])
		return c, serr.NewErr(serr.TErrVersion, dei.Path)
	}
	return c, nil
}

// XXX retry
func (fs *FsEtcd) rename(dei *DirEntInfo, dir *DirInfo, v sp.TQversion, del, from *DirEntInfo, npn path.Tpathname, f sp.Tfence) (spstats.Tcounter, *serr.Err) {
	c := spstats.NewCounter(0)
	opts := from.LeaseOpts()
	d1, r := marshalDirInfo(dir)
	if r != nil {
		return c, r
	}
	fenced := f.PathName != ""
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
	if fenced {
		cmp = append(cmp, clientv3.Compare(clientv3.CreateRevision(f.PathName), "=", int64(f.Epoch)))
		ops1 = append(ops1, clientv3.OpGet(f.PathName))
	}
	if from.LeaseId.IsLeased() {
		ops = append(ops, clientv3.OpPut(fs.leasedkey(from), npn.String(), opts...))
	}
	spstats.Inc(&c, 1)
	resp, err := fs.Clnt().Txn(context.TODO()).If(cmp...).Then(ops...).Else(ops1...).Commit()
	db.DPrintf(db.FSETCD, "Rename dei %v dir %v from %v %v err %v\n", dei, dir, from, resp, err)
	if err != nil {
		return c, serr.NewErrError(err)
	}
	if !resp.Succeeded {
		if len(resp.Responses[0].GetResponseRange().Kvs) != 1 {
			db.DPrintf(db.FSETCD, "rename %v stale", fs.fencekey)
			return c, serr.NewErr(serr.TErrUnreachable, fs.fencekey)
		}
		if fenced && len(resp.Responses[3].GetResponseRange().Kvs) != 1 {
			db.DPrintf(db.FENCEFS, "rename %v f stale", f)
			return c, serr.NewErr(serr.TErrStale, f.PathName)
		}
		if len(resp.Responses[1].GetResponseRange().Kvs) != 1 {
			db.DPrintf(db.FSETCD, "rename from %v doesn't exist\n", from)
			return c, serr.NewErr(serr.TErrNotfound, from.Path)
		}
		if len(resp.Responses[2].GetResponseRange().Kvs) == 1 &&
			int64(v) != resp.Responses[2].GetResponseRange().Kvs[0].Version {
			db.DPrintf(db.FSETCD, "rename %v version mismatch %v %v\n", dei, v, resp.Responses[2])
			return c, serr.NewErr(serr.TErrVersion, dei.Path)
		}
		return c, serr.NewErr(serr.TErrNotfound, del.Path)
	}
	return c, nil
}

// XXX retry
func (fs *FsEtcd) renameAt(deif *DirEntInfo, dirf *DirInfo, vf sp.TQversion, deit *DirEntInfo, dirt *DirInfo, vt sp.TQversion, del, from *DirEntInfo, npn path.Tpathname) (spstats.Tcounter, *serr.Err) {
	c := spstats.NewCounter(0)
	opts := from.LeaseOpts()
	bf, r := marshalDirInfo(dirf)
	if r != nil {
		return c, r
	}
	bt, r := marshalDirInfo(dirt)
	if r != nil {
		return c, r
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

	if from.LeaseId.IsLeased() {
		ops = append(ops, clientv3.OpPut(fs.leasedkey(from), npn.String(), opts...))
	}

	spstats.Inc(&c, 1)
	resp, err := fs.Clnt().Txn(context.TODO()).If(cmp...).Then(ops...).Else(ops1...).Commit()
	db.DPrintf(db.FSETCD, "RenameAt %v %v err %v\n", del, resp, err)
	if err != nil {
		return c, serr.NewErrError(err)
	}
	if !resp.Succeeded {
		if len(resp.Responses[0].GetResponseRange().Kvs) != 1 {
			db.DPrintf(db.FSETCD, "renameat %v stale\n", fs.fencekey)
			return c, serr.NewErr(serr.TErrUnreachable, fs.fencekey)
		}
		if len(resp.Responses[1].GetResponseRange().Kvs) != 1 {
			db.DPrintf(db.FSETCD, "renameat from %v doesn't exist\n", from)
			return c, serr.NewErr(serr.TErrNotfound, from.Path)
		}
		if len(resp.Responses[2].GetResponseRange().Kvs) == 1 &&
			int64(vf) != resp.Responses[2].GetResponseRange().Kvs[0].Version {
			db.DPrintf(db.FSETCD, "renameat %v version mismatch %v %v\n", deif, vf, resp.Responses[2])
			return c, serr.NewErr(serr.TErrVersion, deif.Path)
		}
		if len(resp.Responses[3].GetResponseRange().Kvs) == 1 &&
			int64(vf) != resp.Responses[3].GetResponseRange().Kvs[0].Version {
			db.DPrintf(db.FSETCD, "renameat %v version mismatch %v %v\n", deit, vt, resp.Responses[3])
			return c, serr.NewErr(serr.TErrVersion, deit.Path)
		}
		return c, serr.NewErr(serr.TErrNotfound, del.Path)
	}
	return c, nil
}
