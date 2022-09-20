package wc_test

import (
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/mr"
	"sigmaos/wc"
)

func TestMap(t *testing.T) {
	rdr := strings.NewReader("abc  def's")
	kvs := make([]*mr.KeyValue, 0)
	err := wc.Map("", rdr, func(kv *mr.KeyValue) error {
		kvs = append(kvs, kv)
		return nil
	})
	assert.Nil(t, err, "map")
	for _, kv := range kvs {
		log.Printf("kv %v\n", kv)
	}
}
