package twopc

import (
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	// db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfsd"
)

type Tstate struct {
	t         *testing.T
	s         *fslib.System
	fsl       *fslib.FsLib
	ch        chan bool
	chPresent chan bool
	pid       string
}

func xTestConcurCoord(t *testing.T) {
	const N = 5

	ts := makeTstate(t)
	ch := make(chan bool)
	for r := 0; r < N; r++ {
		go ts.runCoord(t, ch)
	}
	for r := 0; r < N; r++ {
		<-ch
	}
	ts.s.Shutdown(ts.fsl)
}
