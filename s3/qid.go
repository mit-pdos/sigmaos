package fss3

import (
	"hash/fnv"

	"ulambda/fs"
	np "ulambda/ninep"
)

func mkTpath(key np.Path) np.Tpath {
	h := fnv.New64a()
	h.Write([]byte(key.String()))
	return np.Tpath(h.Sum64())
}

func qid(perm np.Tperm, key np.Path) np.Tqid {
	return np.MakeQid(np.Qtype(perm>>np.QTYPESHIFT), np.TQversion(0), mkTpath(key))
}

func mkQids(base *Obj) ([]np.Tqid, fs.FsObj) {
	qids := make([]np.Tqid, 0, len(base.key))
	for i, _ := range base.key {
		if i+1 >= len(base.key) {
			break
		}
		qids = append(qids, qid(np.DMDIR, base.key[0:i+1]))
	}
	qids = append(qids, qid(base.perm, base.key))
	return qids, makeFsObj(base.bucket, base.perm, base.key)
}

func nameiObj(ctx fs.CtxI, bucket string, p np.Path) ([]np.Tqid, fs.FsObj, *np.Err) {
	o := makeObj(bucket, p, np.Tperm(0777))
	if err := o.readHead(fss3); err != nil {
		return nil, nil, err
	}
	qids, fo := mkQids(o)
	return qids, fo, nil
}
