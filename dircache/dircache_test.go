package dircache_test

import (
	"flag"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/dircache"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

var pathname string // e.g., --path "name/ux/~local/" or  "name/schedd/~local/"

func init() {
	flag.StringVar(&pathname, "path", sp.NAMED, "path for file system")
}

func TestCompile(t *testing.T) {
}

func newEntry(n string) (struct{}, error) {
	return struct{}{}, nil
}

func TestDirCache(t *testing.T) {
	ts, err := test.NewTstatePath(t, pathname)
	assert.Nil(t, err)

	dn := filepath.Join(pathname, "d")
	fn := "f"
	err = ts.MkDir(dn, 0777)
	assert.Nil(t, err)

	_, err = ts.PutFile(filepath.Join(dn, fn), 0777, sp.OWRITE, nil)
	assert.Nil(t, err)

	dc := dircache.NewDirCache[struct{}](ts.FsLib, dn, newEntry, nil, db.TEST, db.TEST)

	ns, err := dc.GetEntries()

	assert.Equal(t, ns[0], fn)

	ts.Shutdown()
}
