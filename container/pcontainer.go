package container

import (
	"context"
	"path"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"

	db "sigmaos/debug"
	"sigmaos/perf"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

func StartPContainer(p *proc.Proc, realm string) (*Container, error) {
	db.DPrintf(db.CONTAINER, "dockerContainer %v\n", realm)
	image := "sigmauser"
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	cmd := append([]string{p.Program}, p.Args...)
	db.DPrintf(db.CONTAINER, "ContainerCreate %v %v %v\n", cmd, p.GetEnv(), container.NetworkMode(sp.Conf.Network.MODE))
	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: image,
			Cmd:   cmd,
			Tty:   false,
			Env:   p.GetEnv(),
		},
		&container.HostConfig{
			NetworkMode: container.NetworkMode(sp.Conf.Network.MODE),
			Mounts: []mount.Mount{
				// user bin dir.
				mount.Mount{
					Type:     mount.TypeBind,
					Source:   path.Join("/tmp/sigmaos-bin", realm),
					Target:   path.Join(sp.SIGMAHOME, "bin", "user"),
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
			Privileged: true,
		}, nil, nil, "")
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
	return &Container{ctx, cli, resp.ID, ip, nil}, nil
}
