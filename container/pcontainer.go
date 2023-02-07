package container

import (
	"context"
	"net"
	"strconv"

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

type Tport int

const (
	FPORT Tport = 1112 // Must be used by uprocd for now
	LPORT Tport = 1122
)

func (p Tport) String() string {
	return strconv.Itoa(int(p))
}

func StartPContainer(p *proc.Proc, kernelId, realm string) (*Container, error) {
	db.DPrintf(db.CONTAINER, "dockerContainer %v\n", realm)
	image := "sigmauser"
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	cmd := append([]string{p.Program}, p.Args...)
	db.DPrintf(db.CONTAINER, "ContainerCreate %v %v %v\n", cmd, p.GetEnv(), container.NetworkMode(sp.Conf.Network.MODE))

	pset := nat.PortSet{} // Ports to expose
	pmap := nat.PortMap{} // NAT mappings for exposed ports
	for i := FPORT; i < LPORT; i++ {
		p, err := nat.NewPort("tcp", i.String())
		if err != nil {
			return nil, err
		}
		pset[p] = struct{}{}
		if i != FPORT {
			// XXX for now
			pmap[p] = []nat.PortBinding{{HostPort: i.String()}}
		} else {
			pmap[p] = []nat.PortBinding{{}}
		}
	}

	db.DPrintf(db.CONTAINER, "pset %v pmap %v\n", pset, pmap)

	endpoints := make(map[string]*network.EndpointSettings, 1)
	endpoints["sigmanet"] = &network.EndpointSettings{}
	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image:        image,
			Cmd:          cmd,
			Tty:          false,
			Env:          p.GetEnv(),
			ExposedPorts: pset,
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
			PortBindings: pmap,
		}, &network.NetworkingConfig{
			EndpointsConfig: endpoints,
		}, nil, kernelId+"-uprocd-"+realm)
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
