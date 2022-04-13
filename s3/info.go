package fss3

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
)

func perm(key string) np.Tperm {
	m := np.Tperm(0)
	if key == "" || strings.HasSuffix(key, "/") {
		m = np.DMDIR
	}
	return m
}

type info struct {
	sync.Mutex
	key     np.Path
	perm    np.Tperm
	sz      np.Tlength
	mtime   int64
	dirents map[string]np.Tperm
}

func (i *info) String() string {
	return fmt.Sprintf("key '%v' %v sz %v path %v", i.key, i.perm, i.sz, path(i.key))
}

func makeInfo(key np.Path, perm np.Tperm) *info {
	i := &info{}
	i.perm = perm
	i.key = key
	i.dirents = make(map[string]np.Tperm)
	return i
}

func (i *info) dirEnts() []fs.FsObj {
	i.Lock()
	defer i.Unlock()
	dents := make([]fs.FsObj, 0, len(i.dirents))
	for name, p := range i.dirents {
		dents = append(dents, makeFsObj(p, i.key.Append(name)))
	}
	return dents
}

func makeFsObj(perm np.Tperm, key np.Path) fs.FsObj {
	if perm.IsDir() {
		return makeDir(key.Copy(), perm)
	} else {
		return makeObj(key.Copy(), perm)
	}
}

func (i *info) lookupDirent(name string) fs.FsObj {
	i.Lock()
	defer i.Unlock()
	if p, ok := i.dirents[name]; ok {
		return makeFsObj(p, i.key.Append(name))
	}
	return nil
}

func (i *info) insertDirent(name string, perm np.Tperm) fs.FsObj {
	i.Lock()
	defer i.Unlock()
	if _, ok := i.dirents[name]; ok {
		return nil
	}
	i.dirents[name] = perm
	return makeFsObj(perm, i.key.Append(name))
}

func (i *info) delDirent(name string) {
	i.Lock()
	defer i.Unlock()
	delete(i.dirents, name)
}

func (i *info) Size() np.Tlength {
	return i.sz
}

func (i *info) stat() *np.Stat {
	st := &np.Stat{}
	if len(i.key) > 0 {
		st.Name = i.key.Base()
	} else {
		st.Name = "" // root
	}
	st.Mode = i.perm | np.Tperm(0777)
	st.Qid = qid(i.perm, i.key)
	st.Uid = ""
	st.Gid = ""
	st.Length = i.sz
	st.Mtime = uint32(i.mtime)
	return st
}

// if o.key is prefix of key, include next component of key (unless
// we already seen it
func (i *info) includeNameL(key string) (string, np.Tperm, bool) {
	s := np.Split(key)
	p := perm(key)
	db.DPrintf("FSS3", "s %v i.key '%v' dirents %v\n", s, i.key, i.dirents)
	for i, c := range i.key {
		if c != s[i] {
			return "", p, false
		}
	}
	if len(s) == len(i.key) {
		return "", p, false
	}
	name := s[len(i.key)]
	_, ok := i.dirents[name]
	if ok {
		p = i.perm
	} else {
		if len(s) > len(i.key)+1 {
			p = np.DMDIR
		}
	}
	return name, p, !ok
}

func s3ReadDirL(fss3 *Fss3, k np.Path) (*info, *np.Err) {
	key := k.String()
	maxKeys := 0
	params := &s3.ListObjectsV2Input{
		Bucket: &bucket,
		Prefix: &key,
	}
	i := makeInfo(k, np.DMDIR)
	p := s3.NewListObjectsV2Paginator(fss3.client, params,
		func(o *s3.ListObjectsV2PaginatorOptions) {
			if v := int32(maxKeys); v != 0 {
				o.Limit = v
			}
		})
	for p.HasMorePages() {
		page, err := p.NextPage(context.TODO())
		if err != nil {
			return nil, np.MkErr(np.TErrBadoffset, key)
		}
		for _, obj := range page.Contents {
			if n, p, ok := i.includeNameL(*obj.Key); ok {
				db.DPrintf("FSS30", "incl %v %v\n", n, p)
				i.dirents[n] = p
			}
		}
	}
	i.sz = np.Tlength(len(i.dirents)) // makeup size
	db.DPrintf("FSS3", "s3ReadDirL %v\n", i.dirents)
	cache.insert(k, i)
	return i, nil
}
