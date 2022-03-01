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

func (fss3 *Fss3) makeDir(key []string, t np.Tperm, p *Dir) *Dir {
	o := fss3.makeObj(key, t, p)
	dir := &Dir{}
	dir.Obj = o.(*Obj)
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
	db.DLPrintf("FSS3", "s %v d.key %v dirents %v\n", s, d.key, d.dirents)
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
	db.DLPrintf("FSS3", "Stat Dir: %v\n", d)
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
	key := np.Join(d.key)
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
			db.DLPrintf("FSS3", "Key: %v\n", *obj.Key)
			if n, m, ok := d.includeNameL(*obj.Key); ok {
				db.DLPrintf("FSS3", "incl %v %v\n", n, m)
				if m == np.DMDIR {
					dir := d.fss3.makeDir(append(d.key, n), m, d)
					d.dirents[n] = dir
				} else {
					o1 := d.fss3.makeObj(append(d.key, n), m, d)
					d.dirents[n] = o1.(*Obj)
				}
			}
		}
	}
	d.isRead = true
	return nil
}

func (d *Dir) namei(ctx fs.CtxI, p []string, inodes []fs.FsObj) ([]fs.FsObj, []string, *np.Err) {
	_, err := d.ReadDir(ctx, 0, 0, np.NoV)
	if err != nil {
		return nil, nil, err
	}
	o1, ok := d.lookupDirent(p[0])
	if !ok {
		return inodes, nil, np.MkErr(np.TErrNotfound, p[0])
	}
	inodes = append(inodes, d)
	if len(p) == 1 {
		return inodes, nil, nil
	} else {
		return o1.(*Dir).namei(ctx, p[1:], inodes)
	}
}

func (d *Dir) Lookup(ctx fs.CtxI, p []string) ([]fs.FsObj, []string, *np.Err) {
	db.DLPrintf("FSS3", "%v: lookup %v %v\n", ctx, d, p)
	if !d.Perm().IsDir() {
		return nil, nil, np.MkErr(np.TErrNotDir, d)
	}
	if len(p) == 0 {
		return nil, nil, nil
	}
	return d.namei(ctx, p, []fs.FsObj{})
}

func (d *Dir) ReadDir(ctx fs.CtxI, cursor int, cnt np.Tsize, v np.TQversion) ([]*np.Stat, *np.Err) {
	var dirents []*np.Stat
	db.DLPrintf("FSS3", "readDir: %v\n", d)
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

	d.sz = npcodec.DirSize(dirents)
	return dirents[cursor:], nil
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

// sub directories will be implicitly created; fake write
func (d *Dir) WriteDir(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	return np.Tsize(len(b)), nil
}

// XXX directories don't fully work: there is a fake directory, when
// trying to read it we get an error.  Maybe create . or .. in the
// directory args.Name, to force the directory into existence
func (d *Dir) Create(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, *np.Err) {
	if perm.IsDir() {
		o1 := d.fss3.makeDir(append(d.key, name), np.DMDIR, d)
		return o1, nil
	}
	key := np.Join(append(d.key, name))
	input := &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	_, err := d.fss3.client.PutObject(context.TODO(), input)
	if err != nil {
		return nil, np.MkErr(np.TErrError, err)
	}
	// XXX ignored perm, only files not directories
	d.mu.Lock()
	defer d.mu.Unlock()
	_, ok := d.dirents[name]
	if ok {
		return nil, np.MkErr(np.TErrExists, name)
	}
	o1 := d.Obj.fss3.makeObj(np.Split(key), 0, d)
	d.dirents[name] = o1.(*Obj)
	return o1, nil
}

func (d *Dir) Renameat(ctx fs.CtxI, from string, od fs.Dir, to string) *np.Err {
	return np.MkErr(np.TErrNotSupported, "Renameat")
}

func (d *Dir) Remove(ctx fs.CtxI, name string) *np.Err {
	key := np.Join(d.key) + "/" + name
	input := &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	_, err := d.fss3.client.DeleteObject(context.TODO(), input)
	if err != nil {
		return np.MkErr(np.TErrError, err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.dirents, name)
	return nil
}

func (d *Dir) Rename(ctx fs.CtxI, from, to string) *np.Err {
	return np.MkErr(np.TErrNotSupported, "Rename")
}
