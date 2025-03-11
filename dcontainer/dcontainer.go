// This package provides StartDockerContainer to run [uprocsrv] or
// [spproxysrv] inside a docker container.
package dcontainer

import (
	"context"
	"errors"
	"io"
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

	"sigmaos/dcontainer/cgroup"
	db "sigmaos/debug"
	"sigmaos/proc"
	chunksrv "sigmaos/sched/msched/proc/chunk/srv"
	sp "sigmaos/sigmap"
	"sigmaos/util/linux/mem"
	"sigmaos/util/perf"
)

const (
	CGROUP_PATH_BASE = "/cgroup/system.slice"
)

type DContainer struct {
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

func execInContainer(ctx context.Context, cli *client.Client, containerID string, cmd string) error {
	execResp, err := cli.ContainerExecCreate(ctx, containerID, types.ExecConfig{
		Cmd:          []string{"sh", "-c", cmd},
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		db.DPrintf(db.CONTAINER, "ExecCreate err %v\n", err)
		return err
	}
	attachResp, err := cli.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{})
	if err != nil {
		db.DPrintf(db.CONTAINER, "ExecStart err %v\n", err)
		return err
	}
	defer attachResp.Close()

	io.Copy(os.Stdout, attachResp.Reader)

	execInspect, err := cli.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		db.DPrintf(db.CONTAINER, "ExecInspect err %v\n", err)
		return err
	}
	if execInspect.ExitCode != 0 {
		db.DPrintf(db.CONTAINER, "ExecInspect failure with exit code %v\n", execInspect.ExitCode)
		return errors.New("ExecInspect failure")
	}

	return nil
}

func StartDockerContainer(p *proc.Proc, kernelId string) (*DContainer, error) {
	image := "sigmauser"
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	membytes := int64(mem.GetTotalMem()) * sp.MBYTE

	score := 0
	memswap := int64(0)

	pset := nat.PortSet{} // Ports to expose
	pmap := nat.PortMap{} // NAT mappings for exposed ports
	netmode := "host"
	var endpoints map[string]*network.EndpointSettings
	cmd := append([]string{p.GetProgram()}, p.Args...)
	db.DPrintf(db.CONTAINER, "ContainerCreate %v %v s %v\n", cmd, p.GetEnv(), score)

	db.DPrintf(db.CONTAINER, "Running procd with Docker")
	db.DPrintf(db.CONTAINER, "Realm: %v", p.GetRealm())

	// Set up default mounts.
	mnts := []mount.Mount{
		// user bin dir.
		mount.Mount{
			Type:     mount.TypeBind,
			Source:   chunksrv.PathHostKernel(kernelId),
			Target:   chunksrv.ROOTBINCONTAINER,
			ReadOnly: false,
		},
		// Python shared libraries
		mount.Mount{
			Type:     mount.TypeBind,
			Source:   path.Join("/tmp/pysl"),
			Target:   path.Join("/tmp/pysl"),
			ReadOnly: false,
		},
		// Python mounts
		mount.Mount{
			Type:     mount.TypeBind,
			Source:   path.Join("/tmp", kernelId, "python"),
			Target:   path.Join("/python"), // Parent of upper/work directory must not be an overlay
			ReadOnly: false,
		},
		mount.Mount{
			Type:     mount.TypeBind,
			Source:   path.Join("/tmp/python"),
			Target:   path.Join("/python/lower"),
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

	// If developing locally, mount kernel bins (uproc-trampoline, spproxyd, and
	// procd) from host, since they are excluded from the container image
	// during local dev in order to speed up build times.
	if p.GetBuildTag() == sp.LOCAL_BUILD {
		db.DPrintf(db.CONTAINER, "Mounting kernel bins to user container for local build")
		mnts = append(mnts,
			mount.Mount{
				Type:     mount.TypeBind,
				Source:   filepath.Join("/tmp/sigmaos-procd-bin"),
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
			Runtime:      "runc",
			NetworkMode:  container.NetworkMode(netmode),
			Mounts:       mnts,
			Privileged:   true,
			PortBindings: pmap,
			OomScoreAdj:  score,
		}, &network.NetworkingConfig{
			EndpointsConfig: endpoints,
		}, nil, kernelId+"-procd-"+p.GetPid().String())
	if err != nil {
		db.DPrintf(db.CONTAINER, "ContainerCreate err %v\n", err)
		return nil, err
	}
	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		db.DPrintf(db.CONTAINER, "ContainerStart err %v\n", err)
		return nil, err
	}

	// Set up Python overlay dir
	overlayCmd := "mkdir -p /python/lower /python/upper /python/work /tmp/python && mount -t overlay overlay -o lowerdir=/python/lower,upperdir=/python/upper,workdir=/python/work /tmp/python"
	err = execInContainer(ctx, cli, resp.ID, overlayCmd)
	if err != nil {
		return nil, err
	}

	// Set up OpenBLAS and GCC libraries for numpy
	numpyCmd := "cp /tmp/pysl/* /usr/lib && cp /usr/lib/libgcc_s.so.1 /usr/lib/libgcc_s-a04fdf82.so.1"
	err = execInContainer(ctx, cli, resp.ID, numpyCmd)
	if err != nil {
		return nil, err
	}

	// PIL setup
	pilCmd := "ln -s /usr/lib/libtiff.so.6.1.0 /usr/lib/libtiff-f7c4a081.so.6.0.2 && ln -s /usr/lib/libjpeg.so.8.3.2 /usr/lib/libjpeg-fd78c7ba.so.62.4.0 && ln -s /usr/lib/libopenjp2.so.2.5.2 /usr/lib/libopenjp2-88597bfd.so.2.5.3 && ln -s /usr/lib/libxcb.so.1.1.0 /usr/lib/libxcb-b63aee95.so.1.1.0"
	err = execInContainer(ctx, cli, resp.ID, pilCmd)
	if err != nil {
		return nil, err
	}

	json, err1 := cli.ContainerInspect(ctx, resp.ID)
	if err1 != nil {
		db.DPrintf(db.CONTAINER, "ContainerInspect err %v\n", err)
		return nil, err
	}
	ip := json.NetworkSettings.IPAddress
	db.DPrintf(db.CONTAINER, "Container ID %v", json.ID)

	db.DPrintf(db.CONTAINER, "network setting: ip %v secondaryIPAddrs %v nets %v ", ip, json.NetworkSettings.SecondaryIPAddresses, json.NetworkSettings.Networks)
	cgroupPath := filepath.Join(CGROUP_PATH_BASE, "docker-"+resp.ID+".scope")
	c := &DContainer{
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

func (c *DContainer) GetCPUUtil() (float64, error) {
	st, err := c.cmgr.GetCPUStats(c.cgroupPath)
	if err != nil {
		db.DPrintf(db.ERROR, "Err get cpu stats: %v", err)
		return 0.0, err
	}
	return st.Util, nil
}

func (c *DContainer) SetCPUShares(cpu int64) error {
	s := time.Now()
	err := c.cmgr.SetCPUShares(c.cgroupPath, cpu)
	db.DPrintf(db.SPAWN_LAT, "DContainer.SetCPUShares %v", time.Since(s))
	return err
}

func (c *DContainer) String() string {
	return c.container[:10]
}

func (c *DContainer) Ip() string {
	return c.ip
}

func (c *DContainer) Shutdown() error {
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
