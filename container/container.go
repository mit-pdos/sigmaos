package container

import (
	"context"
	"fmt"
	"os"

	"github.com/docker/docker/client"

	db "sigmaos/debug"
)

//
// exec-container enters here
//

const (
	PROC = "PROC"
)

func ExecContainer() error {
	db.DPrintf(db.CONTAINER, "execContainer: %v\n", os.Args)

	var r error
	switch os.Args[1] {
	default:
		r = fmt.Errorf("ExecContainer: unknown container type: %s", os.Args[1])
	}
	return r
}

type Container struct {
	ctx       context.Context
	cli       *client.Client
	container string
	ip        string
}

func (c *Container) String() string {
	return c.container[:10]
}

func (c *Container) Ip() string {
	return c.ip
}
