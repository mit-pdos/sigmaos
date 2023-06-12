package namedv1

import (
	"hash/fnv"
	"strconv"
	"time"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type Key struct {
	realm sp.Trealm
}

func mkTpath(pn path.Path) sessp.Tpath {
	h := fnv.New64a()
	t := time.Now() // maybe use revision
	h.Write([]byte(pn.String() + t.String()))
	return sessp.Tpath(h.Sum64())
}

func path2key(path sessp.Tpath) string {
	return strconv.FormatUint(uint64(path), 16)
}

func key2path(key string) sessp.Tpath {
	p, err := strconv.ParseUint(key, 16, 64)
	if err != nil {
		db.DFatalf("ParseUint %v err %v\n", key, err)
	}
	return sessp.Tpath(p)
}
