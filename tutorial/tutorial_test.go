package tutorial_test

import (
	"testing"

	//	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/test"
)

func TestStartStop(t *testing.T) {
	ts := test.MakeTstateAll(t)
	db.DPrintf(db.TEST, "Successfully started SigmaOS")
	ts.Shutdown()
}
