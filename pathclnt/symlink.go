package pathclnt

import (
	"log"
	"strings"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/proc"
)

func (fsc *PathClnt) walkSymlink(fid np.Tfid, resolved, left []string) ([]string, *np.Err) {
	// XXX change how we readlink; getfile?
	target, err := fsc.readlink(fid)
	if len(target) == 0 {
		log.Printf("readlink %v %v\n", string(target), err)
	}
	if err != nil {
		return nil, err
	}
	var path []string
	if IsRemoteTarget(target) {
		trest, err := fsc.autoMount(fsc.FidClnt.Lookup(fid).Uname(), target, resolved)
		if err != nil {
			log.Printf("%v: automount %v %v err %v\n", proc.GetName(), resolved, target, err)
			return nil, err
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
func SplitTarget(target string) (string, []string) {
	var server string
	var rest []string

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

func SplitTargetReplicated(target string) ([]string, []string) {
	target = strings.TrimSpace(target)
	targets := strings.Split(target, "\n")
	servers := []string{}
	rest := []string{}
	for _, t := range targets {
		serv, r := SplitTarget(t)
		rest = r
		servers = append(servers, serv)
	}
	return servers, rest
}

func (fsc *PathClnt) autoMount(uname string, target string, path []string) ([]string, *np.Err) {
	db.DLPrintf("FSCLNT", "automount %v to %v\n", target, path)
	var rest []string
	var fid np.Tfid
	var err *np.Err
	if IsReplicated(target) {
		servers, r := SplitTargetReplicated(target)
		rest = r
		fid, err = fsc.Attach(uname, servers, np.Join(path), "")
	} else {
		server, r := SplitTarget(target)
		rest = r
		fid, err = fsc.Attach(uname, []string{server}, np.Join(path), "")
	}
	if err != nil {
		db.DLPrintf("FSCLNT", "Attach error: %v", err)
		return nil, err
	}
	err = fsc.mount(fid, np.Join(path))
	if err != nil {
		return nil, err
	}
	return rest, nil
}
