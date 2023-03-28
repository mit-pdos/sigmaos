package container

import (
	"context"
	"path"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	db "sigmaos/debug"
	"sigmaos/mem"
	"sigmaos/perf"
	"sigmaos/port"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

// Start container for uprocd. If r is nil, don't use overlays.
func StartPContainer(p *proc.Proc, kernelId string, realm sp.Trealm, r *port.Range, up port.Tport, ptype proc.Ttype) (*Container, error) {
	image := "sigmauser"
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	// set overlay network
	net := sp.ROOTREALM.String()
	if r != nil {
		net = "sigmanet-" + realm.String()
		if realm == sp.ROOTREALM {
			net = "sigmanet-testuser"
		}
	}

	score := 0
	swappiness := int64(0)
	if ptype == proc.T_BE {
		score = 1000
		swappiness = 100
	}

	// append uprocd's port
	p.Args = append(p.Args, up.String())
	p.AppendEnv(proc.SIGMANET, net)

	cmd := append([]string{p.Program}, p.Args...)
	db.DPrintf(db.CONTAINER, "ContainerCreate %v %v %v r %v s %v\n", cmd, p.GetEnv(), r, realm, score)

	pset := nat.PortSet{} // Ports to expose
	pmap := nat.PortMap{} // NAT mappings for exposed ports
	netmode := "host"
	var endpoints map[string]*network.EndpointSettings
	if r != nil {
		netmode = "bridge"
		for i := r.Fport; i < r.Lport; i++ {
			p, err := nat.NewPort("tcp", i.String())
			if err != nil {
				return nil, err
			}
			pset[p] = struct{}{}
			pmap[p] = []nat.PortBinding{{}}
		}
		endpoints = make(map[string]*network.EndpointSettings, 1)
		endpoints[net] = &network.EndpointSettings{}
	}

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image:        image,
			Cmd:          cmd,
			Tty:          false,
			Env:          p.GetEnv(),
			ExposedPorts: pset,
		}, &container.HostConfig{
			NetworkMode: container.NetworkMode(netmode),
			Mounts: []mount.Mount{
				// user bin dir.
				mount.Mount{
					Type:     mount.TypeBind,
					Source:   path.Join("/tmp/sigmaos-bin", realm.String()),
					Target:   path.Join(sp.SIGMAHOME, "bin", "user"),
					ReadOnly: true,
				},
				// perf output dir
				mount.Mount{
					Type:     mount.TypeBind,
					Source:   perf.OUTPUT_PATH,
					Target:   perf.OUTPUT_PATH,
					ReadOnly: false,
				},
			},
			Privileged:   true,
			PortBindings: pmap,
			OomScoreAdj:  score,
			Resources: container.Resources{
				// This also allows for GetTotalMem() of swap, if host
				// has swap space
				Memory:           int64(mem.GetTotalMem()) * sp.MBYTE,
				MemorySwappiness: &swappiness,
			},
		}, &network.NetworkingConfig{
			EndpointsConfig: endpoints,
		}, nil, kernelId+"-uprocd-"+realm.String()+"-"+p.GetPid().String())
	if err != nil {
		db.DPrintf(db.CONTAINER, "ContainerCreate err %v\n", err)
		return nil, err
	}
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

	pm := port.MakePortMap(json.NetworkSettings.NetworkSettingsBase.Ports, r)

	db.DPrintf(db.CONTAINER, "network setting: ip %v portmap %v\n", ip, pm)
	return &Container{pm, ctx, cli, resp.ID, ip, nil}, nil
}
