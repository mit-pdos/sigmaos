package exp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"

	db "sigmaos/debug"
	"sigmaos/proc"
	procsrv "sigmaos/sched/msched/proc/clnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

func SpawnViaDocker(p *proc.Proc, parentProcEnv *proc.ProcEnv) error {
	p.Args = append(p.Args, "running-in-docker")
	// Inherit parent proc env
	p.InheritParentProcEnv(parentProcEnv)
	p.GetProcEnv().UseDialProxy = false
	p.FinalizeEnv(parentProcEnv.GetInnerContainerIP(), parentProcEnv.GetOuterContainerIP(), sp.NO_PID)

	image := "cgroups-hotel-imgresize"
	tmpBase := "/tmp"
	perfOutputPath := filepath.Join(tmpBase, perf.OUTPUT_DIR)
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	cmd := append([]string{filepath.Join("/home/sigmaos/bin", p.GetVersionedProgram())}, p.Args...)
	db.DPrintf(db.TEST, "ContainerCreate %v %v", cmd, p.GetEnv())

	// Set up perf mount.
	mnts := []mount.Mount{
		// perf output dir
		mount.Mount{
			Type:     mount.TypeBind,
			Source:   perfOutputPath,
			Target:   perf.OUTPUT_PATH,
			ReadOnly: false,
		},
	}

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: image,
			Cmd:   cmd,
			Tty:   false,
			Env:   p.GetEnv(),
		}, &container.HostConfig{
			Runtime:     "runc",
			NetworkMode: container.NetworkMode("host"),
			Mounts:      mnts,
			Privileged:  true,
		}, &network.NetworkingConfig{}, nil, "cgroups-hotel-imgresize-"+p.GetPid().String())
	if err != nil {
		db.DPrintf(db.TEST, "ContainerCreate err %v\n", err)
		return err
	}
	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		db.DPrintf(db.TEST, "ContainerStart err %v\n", err)
		return err
	}

	json, err1 := cli.ContainerInspect(ctx, resp.ID)
	if err1 != nil {
		db.DPrintf(db.TEST, "ContainerInspect err %v\n", err)
		return err
	}
	ip := json.NetworkSettings.IPAddress
	db.DPrintf(db.TEST, "Container ID %v", json.ID)

	db.DPrintf(db.TEST, "network setting: ip %v secondaryIPAddrs %v nets %v ", ip, json.NetworkSettings.SecondaryIPAddresses, json.NetworkSettings.Networks)
	const SHARE_PER_CORE = 1000
	var cpu int64
	if p.GetType() == proc.T_LC {
		cpu = (SHARE_PER_CORE * int64(p.GetMcpu())) / 1000
	} else {
		// XXX MIN_SHARE?
		cpu = int64(procsrv.BE_SHARES)
	}
	cgroupPath := filepath.Join("/sys/fs/cgroup/system.slice", "docker-"+resp.ID+".scope", "cpu.weight")
	home, err := os.UserHomeDir()
	if err != nil {
		db.DPrintf(db.ERROR, "Err get homedir: %v", err)
		return err
	}
	db.DPrintf(db.TEST, "Running %v with Docker cpu shares %v", p.GetProgram(), cpu)
	// XXX Probably shouldn't hard-code the sigmaos project root path
	SIGMAOS_PROJECT_ROOT := filepath.Join(home, "sigmaos/set-cgroups.sh")
	// Set the cgroups CPU shares for the proc's container
	cmd2 := exec.Command(SIGMAOS_PROJECT_ROOT, cgroupPath, strconv.Itoa(int(cpu)))
	cmd2.Stdout = os.Stdout
	cmd2.Stderr = os.Stderr
	if err := cmd2.Run(); err != nil {
		db.DPrintf(db.ERROR, "Err set cgroups: %v", err)
		return err
	}

	return nil
}
