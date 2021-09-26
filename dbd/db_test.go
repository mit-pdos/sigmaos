package dbd

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/kernel"
	"ulambda/realm"
)

const (
	bin = ".."
	dbd = "name/db/mydb/"
)

type Tstate struct {
	*fslib.FsLib
	t   *testing.T
	s   *kernel.System
	e   *realm.TestEnv
	cfg *realm.RealmConfig
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	ts.t = t

	e := realm.MakeTestEnv(bin)
	cfg, err := e.Boot()
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.e = e
	ts.cfg = cfg
	s := kernel.MakeSystemNamedAddr(bin, cfg.NamedAddr)
	ts.s = s
	ts.FsLib = fslib.MakeFsLibAddr("db_test", cfg.NamedAddr)
	db.Name("db_test")
	return ts
}

type Book struct {
	Author string
	Price  string
	Title  string
}

func TestQuery(t *testing.T) {
	ts := makeTstate(t)

	q := []byte("select * from book where author='Homer';")
	b, err := ts.ReadFile(dbd + "clone")
	sid := string(b)
	err = ts.WriteFile(dbd+sid+"/query", q)
	assert.Nil(t, err, "WriteFile")
	b, err = ts.ReadFile(dbd + sid + "/data")
	assert.Nil(t, err, "Read data")

	var books []Book
	err = json.Unmarshal(b, &books)
	assert.Nil(t, err, "Unmarshal")
	assert.Equal(t, "Odyssey", books[0].Title)

	ts.s.Shutdown()
	ts.e.Shutdown()
}
