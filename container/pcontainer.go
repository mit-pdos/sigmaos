package container

import (
	"context"
	"path"
	"syscall"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"sigmaos/cgroup"
	db "sigmaos/debug"
	"sigmaos/mem"
	"sigmaos/perf"
	"sigmaos/port"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

// Start container for uprocd. If r is nil, don't use overlays.
// func StartPContainer(p *proc.Proc, kernelId string, realm sp.Trealm, r *port.Range, up port.Tport, ptype proc.Ttype) (*Container, error) {
func StartPContainer(p *proc.Proc, kernelId string, r *port.Range, up port.Tport, gvisor bool) (*Container, error) {
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
		runtime = "runsc-debug"
		// XXX switch to non-debug version of the gVisor runtime
		//		runtime = "runsc"
	}

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image:        image,
			Cmd:          cmd,
			Tty:          false,
			Env:          p.GetEnv(),
			ExposedPorts: pset,
		}, &container.HostConfig{
			Runtime:     runtime,
			NetworkMode: container.NetworkMode(netmode),
			Mounts: []mount.Mount{
				// user bin dir.
				mount.Mount{
					Type:   mount.TypeBind,
					Source: path.Join("/tmp/sigmaos-bin"),
					Target: path.Join(sp.SIGMAHOME, "all-realm-bin"),
					//					Source:   path.Join("/tmp/sigmaos-bin", realm.String()),
					//					Target:   path.Join(sp.SIGMAHOME, "bin", "user"),
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

	db.DPrintf(db.CONTAINER, "network setting: ip %v portmap %v\n", ip, pm)
	cgroupPath := path.Join(CGROUP_PATH_BASE, "docker-"+resp.ID+".scope")
	c := &Container{
		PortMap:    pm,
		ctx:        ctx,
		cli:        cli,
		container:  resp.ID,
		cgroupPath: cgroupPath,
		ip:         ip,
		cmgr:       cgroup.NewCgroupMgr(),
	}
	c.cmgr.SetMemoryLimit(c.cgroupPath, membytes, memswap)
	return c, nil
}

func MountRealmBinDir(realm sp.Trealm) error {
	// Mount realm bin directory
	if err := syscall.Mount(path.Join(sp.SIGMAHOME, "all-realm-bin", realm.String()), path.Join(sp.SIGMAHOME, "bin", "user"), "none", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		db.DPrintf(db.ALWAYS, "failed to mount /realm bin dir: %v", err)
		return err
	}
	return nil
}
