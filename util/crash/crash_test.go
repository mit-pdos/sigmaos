package crash

import (
	"testing"

	"github.com/stretchr/testify/assert"
	db "sigmaos/debug"
	"sigmaos/proc"
)

func TestCompile(t *testing.T) {
	es := make([]Event, 0)
	es = append(es, Event{NAMED_PARTITION, 1000, 3333})
	s, err := MakeEvents(es)
	db.DPrintf(db.TEST, "env: %v", string(s))
	proc.SetSigmaFail(string(s))
	s1 := proc.GetSigmaFail()
	err = parseEvents(s1, labels)
	assert.Nil(t, err)
	e, ok := labels[NAMED_PARTITION]
	assert.True(t, ok)
	assert.True(t, e.MaxWait == 1000)
}
