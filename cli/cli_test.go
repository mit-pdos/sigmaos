package main

import (
	"fmt"
	"log"
	sp "sigmaos/sigmap"
	"testing"

	"github.com/stretchr/testify/assert"
)
func TestCompile(t *testing.T) {
	fmt.Printf("start")

	dir := sp.NAMED
	ts, err1 := NewTstatePath(dir)
	assert.Nil(t, err1, "Error New Tstate: %v", err1)	
	sts, err := ts.GetDir(dir)
	if err != nil{
		fmt.Printf("error2")
	}
	log.Printf("%v: %v\n", dir, sp.Names(sts))

	defer ts.Shutdown()
}