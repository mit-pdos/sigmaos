package dbd

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	np "ulambda/ninep"
	"ulambda/test"
)

func TestQuery(t *testing.T) {
	ts := test.MakeTstateAll(t)

	q := []byte("select * from book where author='Homer';")
	b, err := ts.GetFile(DBD + "clone")
	assert.Nil(t, err, "GetFile")
	sid := string(b)
	_, err = ts.SetFile(DBD+sid+"/query", q, np.OWRITE, 0)
	assert.Nil(t, err, "WriteFile")
	b, err = ts.GetFile(DBD + sid + "/data")
	assert.Nil(t, err, "Read data")

	var books []Book
	err = json.Unmarshal(b, &books)
	assert.Nil(t, err, "Unmarshal")
	assert.Equal(t, "Odyssey", books[0].Title)

	ts.Shutdown()
}
