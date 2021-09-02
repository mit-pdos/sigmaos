package stats

import (
	"testing"

	"github.com/stretchr/testify/assert"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/kernel"
)

type Tstate struct {
	*fslib.FsLib
	t *testing.T
	s *kernel.System
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	s := kernel.MakeSystem("..")
	err := s.BootMin()
	if err != nil {
		t.Fatalf("Boot %v\n", err)
	}
	db.Name("stats_test")
	ts.FsLib = fslib.MakeFsLib("statstest")
	ts.s = s
	ts.t = t

	return ts
}

func TestStatsd(t *testing.T) {
	ts := makeTstate(t)

	stats := StatInfo{}
	err := ts.ReadFileJson("name/statsd", &stats)
	assert.Nil(t, err, "statsd")
	assert.Equal(t, Tcounter(1), stats.Nread, "Nread")
	for i := 0; i < 1000; i++ {
		_, err := ts.ReadFile("name/statsd")
		assert.Nil(t, err, "statsd")
	}
	err = ts.ReadFileJson("name/statsd", &stats)
	assert.Nil(t, err, "statsd")
	assert.Equal(t, Tcounter(1002), stats.Nopen, "statsd")

	ts.s.Shutdown()
}
