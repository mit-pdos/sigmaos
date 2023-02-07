package container

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/docker/go-connections/nat"
)

func TestExpose(t *testing.T) {
	ports, err := nat.NewPort("tcp", FPORT+"-"+LPORT)
	assert.Nil(t, err)
	pms, err := nat.ParsePortSpec("0.0.0.0:" + FPORT + "-" + LPORT + ":8112-8113")
	assert.Nil(t, err)
	pmap := nat.PortMap{}
	pmap[ports] = []nat.PortBinding{}
	log.Printf("ports %v pms  %v\n", ports, pms)
}
