package fss3

import (
	"context"
	"sort"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
	"ulambda/npcodec"
)

type Dir struct {
	*Obj
	mu      sync.Mutex
	dirents map[string]fs.FsObj
}

func (fss3 *Fss3) makeDir(key np.Path, t np.Tperm, p *Dir) *Dir {
	o := fss3.makeObj(key, t, p)
	dir := &Dir{}
	dir.Obj = o
	dir.dirents = make(map[string]fs.FsObj)
	return dir
}

func (d *Dir) lookupDirent(name string) (fs.FsObj, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n, ok := d.dirents[name]
	return n, ok
}

// if o.key is prefix of key, include next component of key (unless
// we already seen it
func (d *Dir) includeNameL(key string) (string, np.Tperm, bool) {
	s := np.Split(key)
	m := mode(key)
	db.DPrintf("FSS3", "s %v d.key %v dirents %v\n", s, d.key, d.dirents)
	for i, c := range d.key {
		if c != s[i] {
			return "", m, false
		}
	}
	if len(s) == len(d.key) {
		return "", m, false
	}
	name := s[len(d.key)]
	_, ok := d.dirents[name]
	if ok {
		m = d.Perm()
	} else {
		if len(s) > len(d.key)+1 {
			m = np.DMDIR
		}
	}
	return name, m, !ok
}

func (d *Dir) Stat(ctx fs.CtxI) (*np.Stat, *np.Err) {
	db.DPrintf("FSS3", "Stat Dir: %v\n", d)
	var err *np.Err
	d.mu.Lock()
	read := d.isRead
	d.mu.Unlock()
	if !read {
		_, err = d.fakeStat(ctx, 0, 0, np.NoV)
	}
	return d.stat(), err
}

func (d *Dir) s3ReadDirL() *np.Err {
	key := d.key.String()
	maxKeys := 0
	params := &s3.ListObjectsV2Input{
		Bucket: &bucket,
		Prefix: &key,
	}
	p := s3.NewListObjectsV2Paginator(d.fss3.client, params,
		func(o *s3.ListObjectsV2PaginatorOptions) {
			if v := int32(maxKeys); v != 0 {
				o.Limit = v
			}
		})
	for p.HasMorePages() {
		page, err := p.NextPage(context.TODO())
		if err != nil {
			return np.MkErr(np.TErrBadoffset, key)
		}
		for _, obj := range page.Contents {
			db.DPrintf("FSS3", "Key: %v\n", *obj.Key)
			if n, m, ok := d.includeNameL(*obj.Key); ok {
				db.DPrintf("FSS3", "incl %v %v\n", n, m)
				if m == np.DMDIR {
					dir := d.fss3.makeDir(append(d.key, n), m, d)
					d.dirents[n] = dir
				} else {
					o1 := d.fss3.makeObj(append(d.key, n), m, d)
					d.dirents[n] = o1
				}
			}
		}
	}
	d.isRead = true
	return nil
}

func (d *Dir) namei(ctx fs.CtxI, p np.Path, qids []np.Tqid) ([]np.Tqid, fs.FsObj, np.Path, *np.Err) {
	_, err := d.ReadDir(ctx, 0, 0, np.NoV)
	if err != nil {
		return nil, nil, nil, err
	}
	o1, ok := d.lookupDirent(p[0])
	if !ok {
		return qids, d, nil, np.MkErr(np.TErrNotfound, p[0])
	}
	qids = append(qids, o1.Qid())
	if len(p) == 1 {
		return qids, o1, nil, nil
	} else {
		return o1.(*Dir).namei(ctx, p[1:], qids)
	}
}

func (d *Dir) Lookup(ctx fs.CtxI, p np.Path) ([]np.Tqid, fs.FsObj, np.Path, *np.Err) {
	db.DPrintf("FSS3", "%v: lookup %v %v\n", ctx, d, p)
	if len(p) == 0 {
		return nil, nil, nil, nil
	}
	if !d.Perm().IsDir() {
		return nil, nil, nil, np.MkErr(np.TErrNotDir, d)
	}
	return d.namei(ctx, p, nil)
}

func (d *Dir) ReadDir(ctx fs.CtxI, cursor int, cnt np.Tsize, v np.TQversion) ([]*np.Stat, *np.Err) {
	var dirents []*np.Stat
	db.DPrintf("FSS3", "readDir: %v\n", d)
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.isRead {
		d.s3ReadDirL()
	}
	for _, o1 := range d.dirents {
		st, err := o1.Stat(ctx)
		if err != nil {
			return nil, err
		}
		dirents = append(dirents, st)
	}
	sort.SliceStable(dirents, func(i, j int) bool {
		return dirents[i].Name < dirents[j].Name
	})
	d.sz = npcodec.MarshalSizeDir(dirents)
	if cursor > len(dirents) {
		return nil, nil
	} else {
		return dirents[cursor:], nil
	}
}

// Just read the names of the entries without stat-ing each of one
// them, because stat-ing an entry that is a directory would read that
// subdir too.  Thus, a stat of the root would compute the file
// system.
func (d *Dir) fakeStat(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]*np.Stat, *np.Err) {
	var dirents []*np.Stat
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.isRead {
		d.s3ReadDirL()
	}
	d.sz = np.Tlength(len(d.dirents)) // make up a size
	return dirents, nil
}

func (d *Dir) WriteDir(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	return 0, np.MkErr(np.TErrIsdir, d)
	// return np.Tsize(len(b)), nil
}

func (d *Dir) Create(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, *np.Err) {
	if perm.IsDir() {
		dir := d.fss3.makeDir(append(d.key, name), np.DMDIR, d)
		// create a fake "file" in "dir" to materialize it
		if _, err := dir.Create(ctx, "_._", perm&0777, m); err != nil {
			db.DPrintf("FSS3", "Create x err %v\n", err)
			return nil, err
		}
		d.dirents[name] = dir
		return dir, nil
	}
	key := d.key.Append(name).String()
	db.DPrintf("FSS3", "Create key: %v\n", key)
	input := &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	_, err := d.fss3.client.PutObject(context.TODO(), input)
	if err != nil {
		return nil, np.MkErrError(err)
	}
	// XXX ignored perm, only files not directories
	d.mu.Lock()
	defer d.mu.Unlock()
	_, ok := d.dirents[name]
	if ok {
		return nil, np.MkErr(np.TErrExists, name)
	}
	o := d.Obj.fss3.makeObj(np.Split(key), 0, d)
	d.dirents[name] = o
	if m == np.OWRITE {
		o.setupWriter()
	}
	return o, nil
}

func (d *Dir) Renameat(ctx fs.CtxI, from string, od fs.Dir, to string) *np.Err {
	return np.MkErr(np.TErrNotSupported, "Renameat")
}

func (d *Dir) Remove(ctx fs.CtxI, name string) *np.Err {
	key := d.key.Append(name).String()
	input := &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	db.DPrintf("FSS3", "Delete key: %v\n", key)
	_, err := d.fss3.client.DeleteObject(context.TODO(), input)
	if err != nil {
		return np.MkErrError(err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.dirents, name)
	return nil
}

func (d *Dir) Rename(ctx fs.CtxI, from, to string) *np.Err {
	return np.MkErr(np.TErrNotSupported, "Rename")
}
