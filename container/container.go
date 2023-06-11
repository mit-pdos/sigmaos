package container

import (
	"bufio"
	"context"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"

	db "sigmaos/debug"
	"sigmaos/linuxsched"
	"sigmaos/port"
)

const (
	CGROUP_PATH_BASE = "/cgroup/system.slice"
)

type Container struct {
	*port.PortMap
	ctx          context.Context
	cli          *client.Client
	container    string
	cgroupPath   string
	ip           string
	cpustatf     *os.File
	sysstatf     *os.File
	br           *bufio.Reader
	prevCPUStats cpustats
}

type cpustats struct {
	totalSysUsecs       uint64
	totalContainerUsecs uint64
	util                float64
}

func (c *Container) GetCPUUtil() (float64, error) {
	// Read total CPU time for cgroup.
	t1 := c.getContainerCPUUsecs()
	// Save previous total CPU time for cgroup.
	t0 := c.prevCPUStats.totalContainerUsecs
	// Read total CPU time for the entire system
	s1 := c.getSystemCPUUsecs()
	// Save previous total CPU time for the entire system.
	s0 := c.prevCPUStats.totalSysUsecs
	// Update saved times.
	c.prevCPUStats.totalSysUsecs = s1
	c.prevCPUStats.totalContainerUsecs = t1
	// If this is the first attempt to collect CPU utilization, bail out early
	// (we don't have any "previous" times yet)
	if t0 == 0 {
		return 0.0, nil
	}
	ctrDelta := t1 - t0
	sysDelta := s1 - s0
	// CPU util calculation based on
	// https://github.com/moby/moby/blob/eb131c5383db8cac633919f82abad86c99bffbe5/cli/command/container/stats_helpers.go#L175
	if sysDelta > 0 && ctrDelta > 0 {
		c.prevCPUStats.util = float64(ctrDelta) / float64(sysDelta) * float64(linuxsched.NCores) * 100.0
	} else {
		db.DPrintf(db.ALWAYS, "GetCPUUtil no delta %v %v", sysDelta, ctrDelta)
	}
	db.DPrintf(db.CONTAINER, "GetCPUUtil ctrDelta %v sysDelta %v util %v", ctrDelta, sysDelta, c.prevCPUStats.util)

	return c.prevCPUStats.util, nil
}

func (c *Container) SetCPUShares(cpu int64) error {
	s := time.Now()
	c.setCPUShares(cpu)
	db.DPrintf(db.SPAWN_LAT, "Container.SetCPUShares %v", time.Since(s))
	return nil
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
