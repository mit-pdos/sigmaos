// This package provides StartDockerContainer to run [uprocsrv] or
// [spproxysrv] inside a docker container.
package dcontainer

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
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

const (
	CGROUP_PATH_BASE = "/cgroup/system.slice"
)

type Dcontainer struct {
	*port.PortMap
	overlays     bool
	ctx          context.Context
	cli          *client.Client
	container    string
	cgroupPath   string
	ip           string
	cmgr         *cgroup.CgroupMgr
	prevCPUStats cpustats
}

type cpustats struct {
	totalSysUsecs       uint64
	totalContainerUsecs uint64
	util                float64
}

func StartDockerContainer(p *proc.Proc, kernelId string, overlays bool, gvisor bool) (*Dcontainer, error) {
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

	pset := nat.PortSet{} // Ports to expose
	pmap := nat.PortMap{} // NAT mappings for exposed ports
	up := sp.NO_PORT
	netmode := "host"
	var endpoints map[string]*network.EndpointSettings
	ports := []sp.Tport{port.UPROCD_PORT, port.PUBLIC_HTTP_PORT, port.PUBLIC_NAMED_PORT}
	if overlays {
		db.DPrintf(db.CONTAINER, "Running with overlay ports: %v", ports)
		up = port.UPROCD_PORT
		netmode = "bridge"
		netname := "sigmanet-testuser"
		for _, i := range ports {
			p, err := nat.NewPort("tcp", i.String())
			if err != nil {
				return nil, err
			}
			pset[p] = struct{}{}
			pmap[p] = []nat.PortBinding{{}}
		}
		endpoints = make(map[string]*network.EndpointSettings, 1)
		endpoints[netname] = &network.EndpointSettings{}
	}

	// append uprocd's port
	p.Args = append(p.Args, up.String())
	cmd := append([]string{p.GetProgram()}, p.Args...)
	db.DPrintf(db.CONTAINER, "ContainerCreate %v %v overlays %v s %v\n", cmd, p.GetEnv(), overlays, score)

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
		mount.Mount{
			Type:     mount.TypeBind,
			Source:   path.Join("/tmp/python"),
			Target:   path.Join("/tmp/python"),
			ReadOnly: true,
		},
		// perf output dir
		mount.Mount{
			Type:     mount.TypeBind,
			Source:   perf.OUTPUT_PATH,
			Target:   perf.OUTPUT_PATH,
			ReadOnly: false,
		},
	}

	// If developing locally, mount kernel bins (exec-uproc-rs, spproxyd, and
	// uprocd) from host, since they are excluded from the container image
	// during local dev in order to speed up build times.
	if p.GetBuildTag() == sp.LOCAL_BUILD {
		db.DPrintf(db.CONTAINER, "Mounting kernel bins to user container for local build")
		mnts = append(mnts,
			mount.Mount{
				Type:     mount.TypeBind,
				Source:   filepath.Join("/tmp/sigmaos-uprocd-bin"),
				Target:   filepath.Join(sp.SIGMAHOME, "bin/kernel"),
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

	var pm *port.PortMap
	if overlays {
		pm = port.NewPortMap(json.NetworkSettings.NetworkSettingsBase.Ports, ports)
	}

	db.DPrintf(db.CONTAINER, "network setting: ip %v secondaryIPAddrs %v nets %v portmap %v", ip, json.NetworkSettings.SecondaryIPAddresses, json.NetworkSettings.Networks, pm)
	cgroupPath := filepath.Join(CGROUP_PATH_BASE, "docker-"+resp.ID+".scope")
	c := &Dcontainer{
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

func (c *Dcontainer) GetCPUUtil() (float64, error) {
	st, err := c.cmgr.GetCPUStats(c.cgroupPath)
	if err != nil {
		db.DPrintf(db.ERROR, "Err get cpu stats: %v", err)
		return 0.0, err
	}
	return st.Util, nil
}

func (c *Dcontainer) SetCPUShares(cpu int64) error {
	s := time.Now()
	err := c.cmgr.SetCPUShares(c.cgroupPath, cpu)
	db.DPrintf(db.SPAWN_LAT, "Dcontainer.SetCPUShares %v", time.Since(s))
	return err
}

func (c *Dcontainer) String() string {
	return c.container[:10]
}

func (c *Dcontainer) Ip() string {
	return c.ip
}

func (c *Dcontainer) Shutdown() error {
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
