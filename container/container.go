package container

import (
	"context"
	"encoding/json"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"

	db "sigmaos/debug"
	"sigmaos/linuxsched"
	"sigmaos/port"
)

type Container struct {
	*port.PortMap
	ctx          context.Context
	cli          *client.Client
	container    string
	ip           string
	prevCPUStats *types.CPUStats
}

func (c *Container) SetCPUShares(cpu int64) error {
	resp, err := c.cli.ContainerUpdate(c.ctx, c.container,
		container.UpdateConfig{
			Resources: container.Resources{
				CPUShares: cpu,
			},
		})
	if len(resp.Warnings) > 0 {
		db.DPrintf(db.ALWAYS, "Set CPU shares warnings: %v", resp.Warnings)
	}
	return err
}

func (c *Container) GetCPUUtil() (float64, error) {
	var resp types.ContainerStats
	var err error
	if c.prevCPUStats == nil {
		// Wait for docker to "prime the stats" on the first attempt to read CPU
		// util.
		resp, err = c.cli.ContainerStats(c.ctx, c.container, false)
	} else {
		resp, err = c.cli.ContainerStatsOneShot(c.ctx, c.container)
	}
	if err != nil {
		db.DFatalf("Error ContainerStats: %v", err)
		return 0.0, err
	}
	// Read the response
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		db.DFatalf("Error ReadAll: %v", err)
		return 0.0, err
	}
	resp.Body.Close()
	st := &types.Stats{}
	err = json.Unmarshal(b, st)
	if err != nil {
		db.DFatalf("Error Unmarshal: %v", err)
		return 0.0, err
	}
	if c.prevCPUStats == nil {
		c.prevCPUStats = &st.PreCPUStats
	}
	// CPU util calculation taken from
	// https://github.com/moby/moby/blob/eb131c5383db8cac633919f82abad86c99bffbe5/cli/command/container/stats_helpers.go#L175
	cpuPercent := 0.0
	cpuDelta := float64(st.CPUStats.CPUUsage.TotalUsage) - float64(c.prevCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(st.CPUStats.SystemUsage) - float64(c.prevCPUStats.SystemUsage)
	db.DPrintf(db.CONTAINER, "sysdelta %v cpudelta %v percpuUsage %v\nstats %v", systemDelta, cpuDelta, st.CPUStats.CPUUsage.PercpuUsage, st)
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(linuxsched.NCores) * 100.0
	} else {
		db.DPrintf(db.ALWAYS, "GetCPUUtil no delta %v %v", systemDelta, cpuDelta)
	}
	c.prevCPUStats = &st.CPUStats
	return cpuPercent, nil
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
