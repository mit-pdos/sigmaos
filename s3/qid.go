package fss3

import (
	"context"
	"hash/fnv"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
)

func readHead(fss3 *Fss3, k np.Path) (*info, *np.Err) {
	key := k.String()
	input := &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	result, err := fss3.client.HeadObject(context.TODO(), input)
	if err != nil {
		return nil, np.MkErrError(err)
	}
	i := makeInfo(k, 0777)

	db.DPrintf("FSS3", "readHead: %v %v\n", key, result.ContentLength)

	i.sz = np.Tlength(result.ContentLength)
	if result.LastModified != nil {
		i.mtime = (*result.LastModified).Unix()
	}
	cache.insert(k, i)
	return i, nil
}

func path(key np.Path) np.Tpath {
	h := fnv.New64a()
	h.Write([]byte(key.String()))
	return np.Tpath(h.Sum64())
}

func qid(perm np.Tperm, key np.Path) np.Tqid {
	return np.MakeQid(np.Qtype(perm>>np.QTYPESHIFT), np.TQversion(0), path(key))
}

func mkQids(base *info) ([]np.Tqid, fs.FsObj) {
	qids := make([]np.Tqid, 0, len(base.key))
	for i, _ := range base.key {
		if i+1 >= len(base.key) {
			break
		}
		qids = append(qids, qid(np.DMDIR, base.key[0:i+1]))
	}
	qids = append(qids, qid(base.perm, base.key))
	return qids, makeFsObj(base.perm, base.key)
}

func nameiObj(ctx fs.CtxI, p np.Path) ([]np.Tqid, fs.FsObj, *np.Err) {
	if i := cache.lookup(p); i != nil {
		qids, o := mkQids(i)
		return qids, o, nil
	}
	i, err := readHead(fss3, p)
	if err != nil {
		return nil, nil, err
	}
	qids, o := mkQids(i)
	return qids, o, nil
}
