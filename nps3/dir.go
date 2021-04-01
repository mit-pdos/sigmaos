package nps3

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
	npo "ulambda/npobjsrv"
)

type Dir struct {
	*Obj
	dirents map[string]npo.NpObj
}

func (nps3 *Nps3) makeDir(key []string, t np.Tperm, p *Dir) *Dir {
	nps3.mu.Lock()
	defer nps3.mu.Unlock()
	o := nps3.makeObjL(key, t, p)
	dir := &Dir{o.(*Obj), make(map[string]npo.NpObj)}
	return dir
}

func (d *Dir) lookupDirent(name string) (npo.NpObj, bool) {
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
	db.DLPrintf("NPS3", "s %v d.key %v dirents %v\n", s, d.key, d.dirents)
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
		m = d.t
	} else {
		if len(s) > len(d.key)+1 {
			m = np.DMDIR
		}
	}
	return name, m, !ok
}

func (d *Dir) Stat(ctx npo.CtxI) (*np.Stat, error) {
	db.DLPrintf("NPS3", "Stat Dir: %v\n", d)
	var err error
	d.mu.Lock()
	read := d.isRead
	d.mu.Unlock()
	if !read {
		_, err = d.ReadDir(ctx, 0, 0, np.NoV)
	}
	return d.stat(), err
}

func (d *Dir) s3ReadDirL() error {
	key := np.Join(d.key)
	maxKeys := 0
	params := &s3.ListObjectsV2Input{
		Bucket: &bucket,
		Prefix: &key,
	}
	p := s3.NewListObjectsV2Paginator(d.nps3.client, params,
		func(o *s3.ListObjectsV2PaginatorOptions) {
			if v := int32(maxKeys); v != 0 {
				o.Limit = v
			}
		})
	for p.HasMorePages() {
		page, err := p.NextPage(context.TODO())
		if err != nil {
			return fmt.Errorf("bad offset")
		}
		for _, obj := range page.Contents {
			db.DLPrintf("NPS3", "Key: %v\n", *obj.Key)
			if n, m, ok := d.includeNameL(*obj.Key); ok {
				db.DLPrintf("NPS3", "incl %v %v\n", n, m)
				if m == np.DMDIR {
					dir := d.nps3.makeDir(append(d.key, n), m, d)
					d.dirents[n] = dir
				} else {
					o1 := d.nps3.makeObjL(append(d.key, n), m, d)
					d.dirents[n] = o1.(*Obj)
				}
			}
		}
	}
	d.isRead = true
	return nil
}

func (d *Dir) Lookup(ctx npo.CtxI, p []string) ([]npo.NpObj, []string, error) {
	db.DLPrintf("NPS3", "%v: lookup %v %v\n", ctx, d, p)
	if !d.t.IsDir() {
		return nil, nil, fmt.Errorf("Not a directory")
	}
	_, err := d.ReadDir(ctx, 0, 0, np.NoV)
	if err != nil {
		return nil, nil, err
	}
	o1, ok := d.lookupDirent(p[0])
	if !ok {
		return nil, nil, fmt.Errorf("file not found")
	}
	if len(p) == 1 {
		return []npo.NpObj{o1}, nil, nil
	} else {
		return o1.(*Dir).Lookup(ctx, p[1:])
	}
}

func (d *Dir) ReadDir(ctx npo.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]*np.Stat, error) {
	var dirents []*np.Stat
	db.DLPrintf("NPS3", "readDir: %v\n", d)
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

	d.sz = npcodec.DirSize(dirents)
	return dirents, nil
}

// XXX directories don't fully work: there is a fake directory, when
// trying to read it we get an error.  Maybe create . or .. in the
// directory args.Name, to force the directory into existence
func (d *Dir) Create(ctx npo.CtxI, name string, perm np.Tperm, m np.Tmode) (npo.NpObj, error) {
	if perm.IsDir() {
		o1 := d.nps3.makeDir(append(d.key, name), np.DMDIR, d)
		return o1, nil
	}
	key := np.Join(append(d.key, name))
	input := &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	_, err := d.nps3.client.PutObject(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	// XXX ignored perm, only files not directories
	d.mu.Lock()
	defer d.mu.Unlock()
	_, ok := d.dirents[name]
	if ok {
		return nil, fmt.Errorf("Name exists")
	}
	o1 := d.nps3.MakeObj(np.Split(key), 0, d)
	d.dirents[name] = o1.(*Obj)
	return o1, nil
}
