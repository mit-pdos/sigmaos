package fss3

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/umpc/go-sortedmap"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
)

func perm(key string) np.Tperm {
	m := np.Tperm(0777)
	if key == "" || strings.HasSuffix(key, "/") {
		m = np.DMDIR
	}
	return m
}

type info struct {
	sync.Mutex
	bucket string
	key    np.Path
	perm   np.Tperm
	sz     np.Tlength
	mtime  int64
	dents  *sortedmap.SortedMap
}

func (i *info) String() string {
	return fmt.Sprintf("key '%v' %v sz %v path %v dents %v", i.key, i.perm, i.sz, getPath(i.key), i.dents)
}

func cmp(a, b interface{}) bool {
	if a == b {
		return true
	}
	return false
}

func makeInfo(bucket string, key np.Path, perm np.Tperm) *info {
	i := &info{}
	i.bucket = bucket
	i.perm = perm
	i.key = key
	i.dents = sortedmap.New(100, cmp)
	return i
}

func (i *info) dirents() []fs.FsObj {
	i.Lock()
	defer i.Unlock()
	dents := make([]fs.FsObj, 0, i.dents.Len())
	i.dents.IterFunc(false, func(rec sortedmap.Record) bool {
		dents = append(dents, makeFsObj(i.bucket, rec.Val.(np.Tperm), i.key.Append(rec.Key.(string))))
		return true
	})
	return dents
}

func (i *info) lookupDirent(name string) fs.FsObj {
	i.Lock()
	defer i.Unlock()

	if p, ok := i.dents.Get(name); ok {
		return makeFsObj(i.bucket, p.(np.Tperm), i.key.Append(name))
	}
	return nil
}

func (i *info) insertDirent(name string, perm np.Tperm) fs.FsObj {
	i.Lock()
	defer i.Unlock()
	if _, ok := i.dents.Get(name); ok {
		return nil
	}
	i.dents.Insert(name, perm)
	ni := makeInfo(i.bucket, i.key.Copy().Append(name), perm)
	cache.insert(i.bucket, ni.key, ni)
	o := makeFsObj(i.bucket, perm, ni.key)
	switch t := o.(type) {
	case *Dir:
		t.info = ni
	case *Obj:
		t.info = ni
	}
	return o
}

func (i *info) delDirent(name string) {
	i.Lock()
	defer i.Unlock()
	i.dents.Delete(name)
	cache.delete(i.bucket, i.key.Append(name))
}

func (i *info) Size() np.Tlength {
	return i.sz
}

func (i *info) SetSize(sz np.Tlength) {
	i.sz = sz
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

func s3ReadDirL(fss3 *Fss3, bucket string, k np.Path) (*info, *np.Err) {
	maxKeys := 0
	key := k.String()
	if key != "" {
		key = key + "/"
	}
	params := &s3.ListObjectsV2Input{
		Bucket:    &bucket,
		Prefix:    aws.String(key),
		Delimiter: aws.String("/"),
	}
	i := makeInfo(bucket, k, np.DMDIR)
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
			db.DPrintf("FSS30", "key %v\n", *obj.Key)
			n := strings.TrimPrefix(*obj.Key, key)
			i.dents.Insert(n, np.Tperm(0777))
		}
		for _, obj := range page.CommonPrefixes {
			db.DPrintf("FSS30", "prefix %v\n", *obj.Prefix)
			n := strings.TrimPrefix(*obj.Prefix, key)
			i.dents.Insert(strings.TrimRight(n, "/"), np.DMDIR)
		}
	}
	i.sz = np.Tlength(i.dents.Len()) // makeup size
	db.DPrintf("FSS3", "s3ReadDirL key '%v' dents %v\n", i.key, i.dents)
	cache.insert(bucket, k, i)
	return i, nil
}
