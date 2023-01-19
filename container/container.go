package container

import (
	"context"
	"fmt"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"

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

func (c *Container) Shutdown() error {
	db.DPrintf(db.CONTAINER, "containerwait for %v\n", c)
	statusCh, errCh := c.cli.ContainerWait(c.ctx, c.container, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		db.DPrintf(db.CONTAINER, "ContainerWait err %v\n", err)
		return err
	case st := <-statusCh:
		db.DPrintf(db.CONTAINER, "container %s done status %v\n", c, st)
	}
	out, err := c.cli.ContainerLogs(c.ctx, c.container, types.ContainerLogsOptions{ShowStderr: true, ShowStdout: true})
	if err != nil {
		panic(err)
	}
	stdcopy.StdCopy(os.Stdout, os.Stderr, out)
	removeOptions := types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}
	if err := c.cli.ContainerRemove(c.ctx, c.container, removeOptions); err != nil {
		db.DPrintf(db.CONTAINER, "ContainerRemove %v err %v\n", c, err)
		return err
	}
	return nil
}
