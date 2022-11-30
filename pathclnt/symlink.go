package pathclnt

import (
	"strings"

	db "sigmaos/debug"
	"sigmaos/fcall"
	np "sigmaos/sigmap"
)

func (pathc *PathClnt) walkSymlink1(fid np.Tfid, resolved, left np.Path) (np.Path, *fcall.Err) {
	// XXX change how we readlink; getfile?
	target, err := pathc.readlink(fid)
	db.DPrintf("WALK", "walksymlink1 %v target %v err %v\n", fid, target, err)
	if err != nil {
		return left, err
	}
	var path np.Path
	if IsRemoteTarget(target) {
		err := pathc.autoMount(pathc.FidClnt.Lookup(fid).Uname(), target, resolved)
		if err != nil {
			db.DPrintf("WALK", "automount %v %v err %v\n", resolved, target, err)
			return left, err
		}
		path = append(resolved, left...)
	} else {
		path = append(np.Split(target), left...)
	}
	return path, nil
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
func SplitTarget(target string) (string, np.Path) {
	var server string
	var rest np.Path

	db.DPrintf("WALK", "split %v\n", target)
	if strings.HasPrefix(target, "[") { // IPv6: [::]:port:pubkey:name
		parts := strings.SplitN(target, ":", 5)
		server = parts[0] + ":" + parts[1] + ":" + parts[2] + ":" + parts[3]
		if len(parts[4:]) > 0 && parts[4] != "" {
			rest = np.Split(parts[4])
		}
	} else { // IPv4
		parts := strings.SplitN(target, ":", 4)
		server = parts[0] + ":" + parts[1]
		if len(parts[3:]) > 0 && parts[3] != "" {
			rest = np.Split(parts[3])
		}
	}
	return server, rest
}

func SplitTargetReplicated(target string) (np.Path, np.Path) {
	target = strings.TrimSpace(target)
	targets := strings.Split(target, "\n")
	servers := np.Path{}
	rest := np.Path{}
	for _, t := range targets {
		serv, r := SplitTarget(t)
		rest = r
		servers = append(servers, serv)
	}
	return servers, rest
}

func (pathc *PathClnt) autoMount(uname string, target string, path np.Path) *fcall.Err {
	db.DPrintf("PATHCLNT0", "automount %v to %v\n", target, path)
	var fid np.Tfid
	var err *fcall.Err
	if IsReplicated(target) {
		addrs, r := SplitTargetReplicated(target)
		fid, err = pathc.Attach(uname, addrs, path.String(), r.String())
	} else {
		addr, r := SplitTarget(target)
		db.DPrintf("PATHCLNT0", "Split target: %v", r)
		fid, err = pathc.Attach(uname, []string{addr}, path.String(), r.String())
	}
	if err != nil {
		db.DPrintf("PATHCLNT", "Attach error: %v", err)
		return err
	}
	err = pathc.mount(fid, path.String())
	if err != nil {
		return err
	}
	return nil
}
