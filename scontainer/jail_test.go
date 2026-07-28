package scontainer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The jail waits for binfs by mount point name, and binsrv's BINFSMNT has a
// trailing slash while mountinfo's mount points don't.
func TestIsMounted(t *testing.T) {
	assert.True(t, isMounted("/"), "root")
	assert.True(t, isMounted("/proc/"), "trailing slash")
	assert.False(t, isMounted("/no/such/mount"), "not a mount point")
}
