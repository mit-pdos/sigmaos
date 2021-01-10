package memfs

// Run go test ulambda/memfs

import (
	"fmt"

	"github.com/stretchr/testify/assert"
	// "github.com/stretchr/testify/require"

	"testing"
)

type TestState struct {
	t  *testing.T
	fs *Root
}

func newTest(t *testing.T) *TestState {
	return &TestState{t, MakeRoot()}
}

func TestRoot(t *testing.T) {
	fmt.Printf("TestGetRoot\n")
	ts := newTest(t)
	assert.Equal(t, ts.fs.inode.Inum, RootInum)
}
