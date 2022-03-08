package pathclnt

import (
	"strings"

	db "ulambda/debug"
	np "ulambda/ninep"
)

func (pathc *PathClnt) walkSymlink1(fid np.Tfid, resolved, left np.Path) (np.Path, *np.Err) {
	// XXX change how we readlink; getfile?
	target, err := pathc.readlink(fid)
	db.DLPrintf("WALK", "walksymlink1 %v target %v err %v\n", fid, target, err)
	if err != nil {
		return left, err
	}
	var path np.Path
	if IsRemoteTarget(target) {
		trest, err := pathc.autoMount(pathc.FidClnt.Lookup(fid).Uname(), target, resolved)
		if err != nil {
			db.DLPrintf("WALK", "automount %v %v err %v\n", resolved, target, err)
			return left, err
		}
		path = append(resolved, append(trest, left...)...)
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

// Remote targets separated by '\n'
func IsReplicated(target string) bool {
	return IsRemoteTarget(target) && strings.Contains(target, "\n")
}

// XXX pubkey is unused
func SplitTarget(target string) (string, np.Path) {
	var server string
	var rest np.Path

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

func (pathc *PathClnt) autoMount(uname string, target string, path np.Path) (np.Path, *np.Err) {
	db.DLPrintf("PATHCLNT", "automount %v to %v\n", target, path)
	var rest np.Path
	var fid np.Tfid
	var err *np.Err
	if IsReplicated(target) {
		servers, r := SplitTargetReplicated(target)
		rest = r
		fid, err = pathc.Attach(uname, servers, path.Join(), "")
	} else {
		server, r := SplitTarget(target)
		rest = r
		fid, err = pathc.Attach(uname, []string{server}, path.Join(), "")
	}
	if err != nil {
		db.DLPrintf("PATHCLNT", "Attach error: %v", err)
		return nil, err
	}
	err = pathc.mount(fid, path.Join())
	if err != nil {
		return nil, err
	}
	return rest, nil
}
