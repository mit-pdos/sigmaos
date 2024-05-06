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

	"sigmaos/cgroup"
	"sigmaos/chunksrv"
	db "sigmaos/debug"
	"sigmaos/mem"
	"sigmaos/perf"
	"sigmaos/port"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

// Start outer container for uprocd. If r is nil, don't use overlays.
func StartPContainer(p *proc.Proc, kernelId string, r *port.Range, up sp.Tport, gvisor bool) (*Container, error) {
	image := "sigmauser"
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	membytes := int64(mem.GetTotalMem()) * sp.MBYTE

	score := 0
	memswap := int64(0)
	// TODO: set swap score
	//	if ptype == proc.T_BE {
	//		score = 1000
	//		memswap = membytes
	//	}

	// append uprocd's port
	p.Args = append(p.Args, up.String())

	cmd := append([]string{p.GetProgram()}, p.Args...)
	db.DPrintf(db.CONTAINER, "ContainerCreate %v %v r %v s %v\n", cmd, p.GetEnv(), r, score)

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
		endpoints[p.GetNet()] = &network.EndpointSettings{}
	}

	runtime := "runc"
	if gvisor {
		db.DPrintf(db.CONTAINER, "Running uprocd with gVisor")
		runtime = "runsc-kvm"
	} else {
		db.DPrintf(db.CONTAINER, "Running uprocd with Docker")
	}

	// Set up default mounts.
	mnts := []mount.Mount{
		// user bin dir.
		mount.Mount{
			Type:     mount.TypeBind,
			Source:   chunksrv.PathHostKernel(kernelId),
			Target:   chunksrv.ROOTBINCONTAINER,
			ReadOnly: false,
		},
		// perf output dir
		mount.Mount{
			Type:     mount.TypeBind,
			Source:   perf.OUTPUT_PATH,
			Target:   perf.OUTPUT_PATH,
			ReadOnly: false,
		},
	}

	// If developing locally, mount kernel bins (exec-uproc-rs, sigmaclntd, and
	// uprocd) from host, since they are excluded from the container image
	// during local dev in order to speed up build times.
	if p.GetBuildTag() == sp.LOCAL_BUILD {
		db.DPrintf(db.CONTAINER, "Mounting kernel bins to user container for local build")
		mnts = append(mnts,
			mount.Mount{
				Type:     mount.TypeBind,
				Source:   path.Join("/tmp/sigmaos-uprocd-bin"),
				Target:   path.Join(sp.SIGMAHOME, "bin/kernel"),
				ReadOnly: true,
			},
		)
	}

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image:        image,
			Cmd:          cmd,
			Tty:          false,
			Env:          p.GetEnv(),
			ExposedPorts: pset,
		}, &container.HostConfig{
			Runtime:      runtime,
			NetworkMode:  container.NetworkMode(netmode),
			Mounts:       mnts,
			Privileged:   true,
			PortBindings: pmap,
			OomScoreAdj:  score,
		}, &network.NetworkingConfig{
			EndpointsConfig: endpoints,
		}, nil, kernelId+"-uprocd-"+p.GetPid().String())
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
	db.DPrintf(db.CONTAINER, "Container ID %v", json.ID)

	pm := port.NewPortMap(json.NetworkSettings.NetworkSettingsBase.Ports, r)

	db.DPrintf(db.CONTAINER, "network setting: ip %v secondaryIPAddrs %v nets %v portmap %v", ip, json.NetworkSettings.SecondaryIPAddresses, json.NetworkSettings.Networks, pm)
	cgroupPath := path.Join(CGROUP_PATH_BASE, "docker-"+resp.ID+".scope")
	c := &Container{
		overlays:   p.GetProcEnv().GetOverlays(),
		PortMap:    pm,
		ctx:        ctx,
		cli:        cli,
		container:  resp.ID,
		cgroupPath: cgroupPath,
		ip:         ip,
		cmgr:       cgroup.NewCgroupMgr(),
	}

	if err := c.cmgr.SetMemoryLimit(c.cgroupPath, membytes, memswap); err != nil {
		return nil, err
	}
	return c, nil
}
