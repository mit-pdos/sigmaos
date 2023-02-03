package container

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

const (
	PORT = "1112"
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
	db.DPrintf(db.CONTAINER, "ContainerCreate %v %v %v\n", cmd, p.GetEnv(), container.NetworkMode(sp.Conf.Network.MODE))
	endpoints := make(map[string]*network.EndpointSettings, 1)
	endpoints["sigmanet"] = &network.EndpointSettings{}
	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: image,
			Cmd:   cmd,
			Tty:   false,
			Env:   p.GetEnv(),
			ExposedPorts: nat.PortSet{
				PORT + "/tcp": struct{}{},
				"1113/tcp":    struct{}{},
			},
		}, &container.HostConfig{
			NetworkMode: container.NetworkMode(sp.Conf.Network.MODE),
			PortBindings: nat.PortMap{
				PORT + "/tcp": []nat.PortBinding{
					{
						HostPort: PORT,
					},
				},
				"1113/tcp": []nat.PortBinding{
					{
						HostPort: "1113",
					},
				},
			},
		}, &network.NetworkingConfig{
			EndpointsConfig: endpoints,
		}, nil, "")
	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		db.DPrintf(db.CONTAINER, "ContainerCreate err %v\n", err)
		return nil, err
	}
	json, err1 := cli.ContainerInspect(ctx, resp.ID)
	if err1 != nil {
		return nil, err
	}
	ip := json.NetworkSettings.IPAddress
	return &Container{ctx, cli, resp.ID, ip, nil}, nil
}
