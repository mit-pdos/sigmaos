// This package implements the outer and inter containers.
// StartPContainer starts the outer container running uprocd from
// [uprocsrv].  uprocd calls RunUproc to run a user proc, which
// creates the inner container using the exec-uproc-rs program and
// runs a proc inside of it.
package container

import (
	"context"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
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

func (c *Container) GetCPUUtil() (float64, error) {
	st, err := c.cmgr.GetCPUStats(c.cgroupPath)
	if err != nil {
		db.DPrintf(db.ERROR, "Err get cpu stats: %v", err)
		return 0.0, err
	}
	return st.Util, nil
}

func (c *Container) SetCPUShares(cpu int64) error {
	s := time.Now()
	err := c.cmgr.SetCPUShares(c.cgroupPath, cpu)
	db.DPrintf(db.SPAWN_LAT, "Container.SetCPUShares %v", time.Since(s))
	return err
}

func (c *Container) AssignToRealm(realm sp.Trealm, ptype proc.Ttype) error {
	// If this container will run BE procs, mark it as SCHED_IDLE
	if ptype == proc.T_BE {
		pids, err := c.cmgr.GetPIDs(c.cgroupPath)
		if err != nil {
			return err
		}
		db.DPrintf(db.CONTAINER, "Assign uprocd to realm %v and set SCHED_IDLE: %v pids %v", realm, c.cgroupPath, pids)
		s := time.Now()
		for _, pid := range pids {
			db.DPrintf(db.CONTAINER, "Set %v SCHED_IDLE", pid)
			if err := setSchedPolicy(pid, linuxsched.SCHED_IDLE); err != nil {
				db.DPrintf(db.ERROR, "Err setSchedPolicy: %v", err)
				return err
			}
		}
		db.DPrintf(db.SPAWN_LAT, "Get/Set sched attr %v", time.Since(s))
	}
	if c.overlays && realm != sp.ROOTREALM {
		s := time.Now()
		netns := "sigmanet-" + realm.String()
		db.DPrintf(db.CONTAINER, "Add container %v to net %v", c.container, netns)
		if err := c.cli.NetworkConnect(c.ctx, netns, c.container, &network.EndpointSettings{}); err != nil {
			db.DFatalf("Error NetworkConnect: %v", err)
		}
		db.DPrintf(db.SPAWN_LAT, "Add to overlay network %v", time.Since(s))
		s = time.Now()
		rootnetns := "sigmanet-testuser"
		db.DPrintf(db.CONTAINER, "Remove container %v from net %v", c.container, netns)
		if err := c.cli.NetworkDisconnect(c.ctx, rootnetns, c.container, true); err != nil {
			db.DFatalf("Error NetworkConnect: %v", err)
		}
		db.DPrintf(db.SPAWN_LAT, "Remove from overlay network %v", time.Since(s))
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
