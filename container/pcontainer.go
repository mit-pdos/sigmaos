package container

import (
	"context"
	"net"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
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
			Mounts: []mount.Mount{
				mount.Mount{
					Type:     mount.TypeBind,
					Source:   PERF_MOUNT,
					Target:   PERF_MOUNT,
					ReadOnly: false,
				},
			},
			PortBindings: nat.PortMap{
				// let host decide on port
				PORT + "/tcp": []nat.PortBinding{{}},
				"1113/tcp":    []nat.PortBinding{{}},
			},
		}, &network.NetworkingConfig{
			EndpointsConfig: endpoints,
		}, nil, "")
	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		db.DPrintf(db.CONTAINER, "ContainerStart err %v\n", err)
		return nil, err
	}
	json, err1 := cli.ContainerInspect(ctx, resp.ID)
	if err1 != nil {
		db.DPrintf(db.CONTAINER, "ContainerInspect err %v\n", err)
		return nil, err
	}
	ip := json.NetworkSettings.IPAddress
	port := ""
	ports := json.NetworkSettings.NetworkSettingsBase.Ports["1112/tcp"]
	for _, p := range ports {
		ip := net.ParseIP(p.HostIP)
		if ip.To4() != nil {
			port = p.HostPort
			break
		}
	}

	db.DPrintf(db.CONTAINER, "network setting: ip %v hostport %v\n", ip, port)
	return &Container{ctx, cli, resp.ID, ip, port, nil}, nil
}
