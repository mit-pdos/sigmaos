package container

import (
	"context"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"

	"sigmaos/cgroup"
	db "sigmaos/debug"
	"sigmaos/linuxsched"
	"sigmaos/port"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
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
	cmgr         *cgroup.CgroupMgr
	pid          int
	prevCPUStats cpustats
}

type cpustats struct {
	totalSysUsecs       uint64
	totalContainerUsecs uint64
	util                float64
}

func (c *Container) GetCPUUtil() (float64, error) {
	st, err := c.cmgr.GetCPUStats(c.cgroupPath)
	if err != nil {
		db.DFatalf("Err get cpu stats: %v", err)
	}
	return st.Util, nil
}

func (c *Container) SetCPUShares(cpu int64) error {
	s := time.Now()
	c.cmgr.SetCPUShares(c.cgroupPath, cpu)
	db.DPrintf(db.SPAWN_LAT, "Container.SetCPUShares %v", time.Since(s))
	return nil
}

func (c *Container) AssignToRealm(realm sp.Trealm, ptype proc.Ttype) error {
	// If this container will run BE procs, mark it as SCHED_IDLE
	if ptype == proc.T_BE {
		db.DPrintf(db.CONTAINER, "Assign uprocd to realm %v and set SCHED_IDLE: %v pid %v", realm, c.cgroupPath, c.pid)
		s := time.Now()
		if err := setSchedPolicy(c.pid, linuxsched.SCHED_IDLE); err != nil {
			db.DFatalf("Err setSchedPolicy: %v", err)
			return err
		}
		db.DPrintf(db.SPAWN_LAT, "[%v] Get/Set sched attr %v", time.Since(s))
	}
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
