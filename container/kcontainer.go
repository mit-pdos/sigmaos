package container

import (
	"context"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

//
// Start kernel inside a docker container
//

const (
	SIGMAKIMAGE = "sigmaos"
	HOME        = "/home/sigmaos"
	HOSTTMP     = "/tmp/sigmaos"
)

func StartKContainer(yml string, nameds sp.Taddrs, env []string) (*Container, error) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.CONTAINER, "start container %v %v\n", nameds, env)
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: SIGMAKIMAGE,
		Cmd:   []string{"bin/linux/bootkernel", yml, nameds.String()},
		Tty:   false,
		Env:   env,
	}, &container.HostConfig{
		// This is bad idea in general because it requires to give rw
		// permission on host to privileged daemon.  But maybe ok in
		// our case where kernel is trusted as is. XXX Use different
		// image for user procs.
		Binds: []string{
			"/var/run/docker.sock:/var/run/docker.sock",
			os.Getenv("HOME") + "/.aws" + ":/home/sigmaos/.aws",
			HOSTTMP + ":" + HOSTTMP,
		},
	}, nil, nil, "")
	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return nil, err
	}
	json, err1 := cli.ContainerInspect(ctx, resp.ID)
	if err1 != nil {
		return nil, err
	}
	ip := json.NetworkSettings.IPAddress
	db.DPrintf(db.CONTAINER, "Booting %s %s at %s...\n", SIGMAKIMAGE, resp.ID[:10], ip)
	return &Container{ctx, cli, resp.ID, ip}, nil
}
