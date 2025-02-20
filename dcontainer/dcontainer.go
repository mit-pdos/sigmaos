// This package provides StartDockerContainer to run [uprocsrv] or
// [spproxysrv] inside a docker container.
package dcontainer

import (
	"context"
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

func copyFile(src string, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}
	return nil
}

func copyLib(src string, dst string) error {
	err := os.MkdirAll(dst, 0777)
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			err = copyLib(srcPath, dstPath)
			if err != nil {
				return err
			}
		} else {
			copyFile(srcPath, dstPath)
		}
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

	// realmName := p.GetRealm()
	// realmDir := filepath.Join("/tmp/python", realmName.String())
	// _, err = os.Stat(realmDir)
	// if err != nil {
	// 	if os.IsNotExist(err) {
	// 		// Create the directory
	// 		err = os.MkdirAll(realmDir, 0777)
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 		// Copy over all lib contents
	// 		err = copyLib("/tmp/python/Lib", filepath.Join(realmDir, "Lib"))
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 		err = copyLib("/tmp/python/build", filepath.Join(realmDir, "build"))
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 		err = copyLib("/tmp/python/Modules", filepath.Join(realmDir, "build"))
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 	} else {
	// 		return nil, err
	// 	}
	// }

	// // Copy over changeable content
	// os.RemoveAll(filepath.Join(realmDir, "pyproc/"))
	// os.RemoveAll(filepath.Join(realmDir, "ld_fstatat.so"))
	// os.RemoveAll(filepath.Join(realmDir, "clntlib.so"))
	// os.RemoveAll(filepath.Join(realmDir, "Lib", "splib.py"))
	// err = copyLib("/tmp/python/pyproc", filepath.Join(realmDir, "pyproc"))
	// if err != nil {
	// 	return nil, err
	// }
	// err = copyFile("/tmp/python/ld_fstatat.so", filepath.Join(realmDir, "ld_fstatat.so"))
	// if err != nil {
	// 	return nil, err
	// }
	// err = copyFile("/tmp/python/clntlib.so", filepath.Join(realmDir, "clntlib.so"))
	// if err != nil {
	// 	return nil, err
	// }
	// err = copyFile("/tmp/python/Lib/splib.py", filepath.Join(realmDir, "Lib", "splib.py"))
	// if err != nil {
	// 	return nil, err
	// }

	// Set up default mounts.
	mnts := []mount.Mount{
		// user bin dir.
		mount.Mount{
			Type:     mount.TypeBind,
			Source:   chunksrv.PathHostKernel(kernelId),
			Target:   chunksrv.ROOTBINCONTAINER,
			ReadOnly: false,
		},
		// Python dirs
		mount.Mount{
			Type:     mount.TypeBind,
			Source:   path.Join("/tmp/python"), // realmDir,
			Target:   path.Join("/tmp/python"),
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
