package stats_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/realm"
	"ulambda/stats"
)

const (
	bin = ".."
)

type Tstate struct {
	*fslib.FsLib
	t   *testing.T
	e   *realm.TestEnv
	cfg *realm.RealmConfig
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}

	e := realm.MakeTestEnv(bin)
	cfg, err := e.Boot()
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	ts.e = e
	ts.cfg = cfg

	db.Name("stats_test")
	ts.FsLib = fslib.MakeFsLibAddr("statstest", cfg.NamedAddr)
	ts.t = t

	return ts
}

func TestStatsd(t *testing.T) {
	ts := makeTstate(t)

	st := stats.StatInfo{}
	err := ts.ReadFileJson("name/statsd", &st)
	assert.Nil(t, err, "statsd")
	assert.NotEqual(t, stats.Tcounter(0), st.Nread, "Nread")
	for i := 0; i < 1000; i++ {
		_, err := ts.ReadFile("name/statsd")
		assert.Nil(t, err, "statsd")
	}
	err = ts.ReadFileJson("name/statsd", &st)
	assert.Nil(t, err, "statsd")
	assert.Greater(t, st.Nopen, stats.Tcounter(1000), "statsd")

	ts.e.Shutdown()
}
