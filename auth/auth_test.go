package auth_test

import (
	"testing"

	//	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/test"
)

func TestStartStop(t *testing.T) {
	rootts := test.NewTstateWithRealms(t)
	db.DPrintf(db.TEST, "Started successfully")
	rootts.Shutdown()
}
