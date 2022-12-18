package sigmap

import (
	"strings"
)

type Tmount struct {
	Mnt string
}

func Address(mnt Tmount) string {
	targets := strings.Split(string(mnt.Mnt), "\n")
	if strings.HasPrefix(targets[0], "[") {
		parts := strings.SplitN(targets[0], ":", 6)
		return "[" + parts[0] + ":" + parts[1] + ":" + parts[2] + "]" + ":" + parts[3]
	} else { // IPv4
		parts := strings.SplitN(targets[0], ":", 4)
		return parts[0] + ":" + parts[1]
	}
}

func MkMountService(srvaddrs []string) Tmount {
	targets := []string{}
	for _, addr := range srvaddrs {
		targets = append(targets, addr+":pubkey")
	}
	return Tmount{strings.Join(targets, "\n")}
}

func MkMountServer(addr string) Tmount {
	return MkMountService([]string{addr})
}

func MkMountTree(mnt Tmount, tree string) Tmount {
	target := []string{string(mnt.Mnt), tree}
	return Tmount{strings.Join(target, ":")}
}
