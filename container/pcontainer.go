package container

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	db "sigmaos/debug"
	"sigmaos/proc"
)

func StartPContainer(p *proc.Proc, realm string) (*Container, error) {
	db.DPrintf(db.CONTAINER, "dockerContainer %v\n", realm)
	image := "sigmauser"
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	cmd := append([]string{p.Program}, p.Args...)
	db.DPrintf(db.CONTAINER, "ContainerCreate %v %v\n", cmd, p.GetEnv())
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: image,
		Cmd:   cmd,
		Tty:   false,
		Env:   p.GetEnv(),
	}, nil, nil, nil, "")
	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		db.DPrintf(db.CONTAINER, "ContainerCreate err %v\n", err)
		return nil, err
	}
	return &Container{ctx, cli, resp.ID, ""}, nil
}
