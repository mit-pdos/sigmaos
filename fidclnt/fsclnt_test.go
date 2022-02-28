package fidclnt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTarget(t *testing.T) {
	assert.Equal(t, true, IsRemoteTarget("192.168.2.114:40043:pubkey"))
	assert.Equal(t, true, IsRemoteTarget("192.168.2.114:40043:pubkey:"))
	assert.Equal(t, true, IsRemoteTarget("192.168.2.114:40043:pubkey:xxx"))
	assert.Equal(t, false, IsRemoteTarget("192.168.2.114:m-0/r-0"))

	server, rest := SplitTarget("192.168.2.114:40043:pubkey")
	assert.Equal(t, "192.168.2.114:40043", server)
	assert.Equal(t, 0, len(rest))

	server, rest = SplitTarget("192.168.2.114:40043:pubkey:")
	assert.Equal(t, "192.168.2.114:40043", server)
	assert.Equal(t, 0, len(rest))

	server, rest = SplitTarget("192.168.2.114:40043:pubkey:m-0:r-0")
	assert.Equal(t, "192.168.2.114:40043", server)
	assert.Equal(t, 1, len(rest))

	server, rest = SplitTarget("192.168.2.114:40043:pubkey:m-0/r-0")
	assert.Equal(t, "192.168.2.114:40043", server)
	assert.Equal(t, 2, len(rest))

	server, rest = SplitTarget("[::]:40043:pubkey:m-0/r-0")
	assert.Equal(t, "[::]:40043", server)
	assert.Equal(t, 2, len(rest))
}
