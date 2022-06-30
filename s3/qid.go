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

func mkObjs(base *Obj) ([]fs.FsObj, fs.FsObj) {
	os := make([]fs.FsObj, 0, len(base.key))
	for i, _ := range base.key {
		if i+1 >= len(base.key) {
			break
		}
		os = append(os, makeFsObj(base.bucket, np.DMDIR, base.key[0:i+1]))
	}
	o := makeFsObj(base.bucket, base.perm, base.key)
	os = append(os, o)
	return os, o
}

func nameiObj(ctx fs.CtxI, bucket string, p np.Path) ([]fs.FsObj, fs.FsObj, *np.Err) {
	o := makeObj(bucket, p, np.Tperm(0777))
	if err := o.readHead(fss3); err != nil {
		return nil, nil, err
	}
	os, fo := mkObjs(o)
	return os, fo, nil
}
