package wc_test

import (
	"log"
	"strings"
	"testing"

	"ulambda/wc"
)

func TestMap(t *testing.T) {
	rdr := strings.NewReader("abc  def's")
	kva := wc.Map("", rdr)
	log.Printf("kvas %v\n", kva)
}
