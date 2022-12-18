package sigmap

import (
	"strings"

	"sigmaos/path"
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

// Replicated: >1 of IPv6/IPv4, separated by a '\n'
// IPv6: [::]:port:pubkey:name
// IPv4: host:port:pubkey[:path]
func IsRemoteTarget(target string) bool {
	targets := strings.Split(target, "\n")
	if strings.HasPrefix(targets[0], "[") {
		parts := strings.SplitN(targets[0], ":", 6)
		return len(parts) >= 5
	} else { // IPv4
		parts := strings.SplitN(targets[0], ":", 4)
		return len(parts) >= 3
	}
}

// Assume IsRemoteTarget(target) is true
func TargetIp(target string) string {
	targets := strings.Split(target, "\n")
	parts := strings.Split(targets[0], ":")
	return parts[0]
}

// Remote targets separated by '\n'
func IsReplicated(target string) bool {
	return IsRemoteTarget(target) && strings.Contains(target, "\n")
}

// XXX pubkey is unused
func SplitTarget(target string) (string, path.Path) {
	var server string
	var rest path.Path

	if strings.HasPrefix(target, "[") { // IPv6: [::]:port:pubkey:name
		parts := strings.SplitN(target, ":", 5)
		server = parts[0] + ":" + parts[1] + ":" + parts[2] + ":" + parts[3]
		if len(parts[4:]) > 0 && parts[4] != "" {
			rest = path.Split(parts[4])
		}
	} else { // IPv4
		parts := strings.SplitN(target, ":", 4)
		server = parts[0] + ":" + parts[1]
		if len(parts[3:]) > 0 && parts[3] != "" {
			rest = path.Split(parts[3])
		}
	}
	return server, rest
}

func SplitTargetReplicated(target string) (path.Path, path.Path) {
	target = strings.TrimSpace(target)
	targets := strings.Split(target, "\n")
	servers := path.Path{}
	rest := path.Path{}
	for _, t := range targets {
		serv, r := SplitTarget(t)
		rest = r
		servers = append(servers, serv)
	}
	return servers, rest
}
