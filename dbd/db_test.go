package dbd

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"ulambda/kernel"
)

type Tstate struct {
	*kernel.System
	t *testing.T
}

func makeTstate(t *testing.T) *Tstate {
	ts := &Tstate{}
	ts.t = t
	ts.System = kernel.MakeSystemAll("db_test", "..", 0)
	return ts
}

func TestQuery(t *testing.T) {
	ts := makeTstate(t)

	q := []byte("select * from book where author='Homer';")
	b, err := ts.GetFile(DBD + "clone")
	assert.Nil(t, err, "GetFile")
	sid := string(b)
	_, err = ts.SetFile(DBD+sid+"/query", q, 0)
	assert.Nil(t, err, "WriteFile")
	b, err = ts.GetFile(DBD + sid + "/data")
	assert.Nil(t, err, "Read data")

	var books []Book
	err = json.Unmarshal(b, &books)
	assert.Nil(t, err, "Unmarshal")
	assert.Equal(t, "Odyssey", books[0].Title)

	ts.Shutdown()
}
