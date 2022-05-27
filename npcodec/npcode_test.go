package npcodec

import (
	"testing"

	"github.com/stretchr/testify/assert"

	np "ulambda/ninep"
)

func TestPutfile(t *testing.T) {
	b := []byte("hello")
	fence := np.Tfence{np.Tfenceid{36, 2}, 7}
	msg := np.Tputfile{1, np.OWRITE, 0777, 101, []string{"f"}, b}
	fcall := np.MakeFcall(msg, 13, nil, &np.Tinterval{1, 2}, fence)
	frame, error := marshal(fcall)
	assert.Nil(t, error)
	fcall1 := &np.Fcall{}
	error = unmarshal(frame, fcall1)
	assert.Nil(t, error)
	assert.Equal(t, fcall1, fcall, "fcall")
}
