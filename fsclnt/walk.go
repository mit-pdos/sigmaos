package fsclnt

import (
	"errors"
	"fmt"
	"io"
	"log"
	"strings"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
	"ulambda/protclnt"
)

const (
	MAXSYMLINK = 4
)

func (fsc *FsClient) WalkManyUmount(p []string, resolve bool, w Watch) (np.Tfid, error) {
	var fid np.Tfid
	for {
		f, err := fsc.walkMany(p, resolve, w)
		db.DLPrintf("FSCLNT", "walkMany %v -> %v %v\n", p, f, err)
		if err == io.EOF {
			p := fsc.path(f)
			if p == nil { // schedd triggers this; don't know why
				return np.NoFid, err
			}
			fid2, e := fsc.mount.umount(p.cname)
			if e != nil {
				return f, e
			}
			fsc.detachChannel(fid2)
			continue
		}
		if err != nil {
			return f, err
		}
		fid = f
		break
	}
	return fid, nil
}

func (fsc *FsClient) walkMany(path []string, resolve bool, w Watch) (np.Tfid, error) {
	for i := 0; i < MAXSYMLINK; i++ {
		fid, todo, err := fsc.walkOne(path, w)
		if err != nil {
			return fid, err
		}
		qid := fsc.path(fid).lastqid()

		// if todo == 0 and !resolve, don't resolve symlinks, so
		// that the client can remove a symlink
		if qid.Type&np.QTSYMLINK == np.QTSYMLINK && (todo > 0 ||
			(todo == 0 && resolve)) {
			path, err = fsc.walkSymlink(fid, path, todo)
			if err != nil {
				return fid, err
			}
		} else {
			return fid, err
		}
	}
	return np.NoFid, errors.New("too many iterations")
}

// Walk to parent directory, and check if name is there.  If it is, return entry.
// Otherwise, set watch based on directory's version number
func (fsc *FsClient) setWatch(fid1, fid2 np.Tfid, p []string, r []string, w Watch) (*np.Rwalk, error) {
	db.DLPrintf("FSCLNT", "Watch %v %v\n", p, r)
	fid3 := fsc.allocFid()
	dir := r[0 : len(r)-1]
	reply, err := fsc.clnt(fid1).Walk(fid1, fid3, dir)
	if err != nil {
		return nil, err
	}
	fsc.addFid(fid3, fsc.path(fid1).copyPath())
	fsc.path(fid3).addn(reply.Qids, dir)

	reply, err = fsc.clnt(fid3).Walk(fid3, fid2, []string{r[len(r)-1]})
	if err == nil {
		return reply, nil
	}

	go func(pc *protclnt.ProtClnt, version np.TQversion) {
		db.DLPrintf("FSCLNT", "Watch set %v %v %v\n", p, r[len(r)-1], version)
		err := pc.Watch(fid3, []string{r[len(r)-1]}, version)
		db.DLPrintf("FSCLNT", "Watch returns %v %v\n", p, err)
		fsc.clunkFid(fid3)
		w(np.Join(p), err)
	}(fsc.clnt(fid3), fsc.path(fid3).lastqid().Version)
	return nil, nil
}

func (fsc *FsClient) walkOne(path []string, w Watch) (np.Tfid, int, error) {
	fid, rest := fsc.mount.resolve(path)
	if fid == np.NoFid {
		db.DLPrintf("FSCLNT", "walkOne: mount -> unknown fid\n")
		if fsc.mount.hasExited() {
			return np.NoFid, 0, io.EOF
		}
		return np.NoFid, 0, errors.New("unknown file")
	}
	db.DLPrintf("FSCLNT", "walkOne: mount -> %v %v\n", fid, rest)
	fid1, err := fsc.clone(fid)
	if err != nil {
		return fid, 0, err
	}
	defer fsc.clunkFid(fid1)
	fid2 := fsc.allocFid()
	first, union := IsUnion(rest)
	var reply *np.Rwalk
	todo := 0
	if union {
		reply, err = fsc.walkUnion(fsc.clnt(fid1), fid1, fid2,
			first, rest[len(first)])
		rest = rest[len(first)+1:]
		todo = len(rest)
		if err != nil {
			return np.NoFid, 0, err
		}
	} else {
		reply, err = fsc.clnt(fid1).Walk(fid1, fid2, rest)
		if err != nil {
			if w != nil && strings.HasPrefix(err.Error(), "file not found") {
				var err1 error
				reply, err1 = fsc.setWatch(fid1, fid2, path, rest, w)
				if err1 != nil {
					// couldn't walk to parent dir
					return np.NoFid, 0, err1
				}
				if err1 == nil && reply == nil {
					// entry is still not in parent dir
					return np.NoFid, 0, err
				}
				// entry now exists
			} else {
				return np.NoFid, 0, err
			}
		}
		todo = len(rest) - len(reply.Qids)
		db.DLPrintf("FSCLNT", "walkOne rest %v -> %v %v", rest, reply.Qids, todo)
	}
	fsc.addFid(fid2, fsc.path(fid1).copyPath())
	fsc.path(fid2).addn(reply.Qids, rest)
	return fid2, todo, nil
}

func (fsc *FsClient) walkSymlink(fid np.Tfid, path []string, todo int) ([]string, error) {
	// XXX change how we readlink; getfile?
	target, err := fsc.Readlink(fid)
	if len(target) == 0 {
		log.Printf("readlink %v %v\n", string(target), err)
	}
	if err != nil {
		return nil, err
	}
	i := len(path) - todo
	rest := path[i:]
	if IsRemoteTarget(target) {
		trest, err := fsc.autoMount(target, path[:i])
		if err != nil {
			return nil, err
		}
		db.DLPrintf("FSCLNT", "rest = %v trest %v (%v)\n", rest, trest, len(trest))
		path = append(path[:i], append(trest, rest...)...)
	} else {
		path = append(np.Split(target), rest...)
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

func (fsc *FsClient) autoMount(target string, path []string) ([]string, error) {
	db.DLPrintf("FSCLNT", "automount %v to %v\n", target, path)
	var rest []string
	var fid np.Tfid
	var err error
	if IsReplicated(target) {
		servers, r := SplitTargetReplicated(target)
		rest = r
		fid, err = fsc.AttachReplicas(servers, np.Join(path), "")
	} else {
		server, r := SplitTarget(target)
		rest = r
		fid, err = fsc.Attach(server, np.Join(path), "")
	}
	if err != nil {
		db.DLPrintf("FSCLNT", "Attach error: %v", err)
		return nil, err
	}
	return rest, fsc.Mount(fid, np.Join(path))
}

func IsUnion(path []string) ([]string, bool) {
	for i, c := range path {
		if strings.HasPrefix(c, "~") {
			return path[:i], true
		}
	}
	return nil, false
}

func (fsc *FsClient) walkUnion(pc *protclnt.ProtClnt, fid, fid2 np.Tfid, dir []string, q string) (*np.Rwalk, error) {
	db.DLPrintf("FSCLNT", "Walk union: %v %v\n", dir, q)
	fid3 := fsc.allocFid()
	reply, err := pc.Walk(fid, fid3, dir)
	if err != nil {
		return nil, err
	}
	reply, err = fsc.unionLookup(fsc.clnt(fid), fid3, fid2, q)
	if err != nil {
		return nil, err
	}
	db.DLPrintf("FSCLNT", "unionLookup -> %v %v", fid2, reply.Qids)
	return reply, nil
}

func (fsc *FsClient) unionMatch(q, name string) bool {
	db.DLPrintf("FSCLNT", "unionMatch %v %v\n", q, name)
	switch q {
	case "~any":
		return true
	case "~ip":
		ip, err := LocalIP()
		if err != nil {
			return false
		}
		parts := strings.Split(ip, ":")
		if parts[0] == ip {
			return true
		}
		return false
	default:
		return true
	}
	return true
}

func (fsc *FsClient) unionScan(pc *protclnt.ProtClnt, fid, fid2 np.Tfid, dirents []*np.Stat, q string) (*np.Rwalk, error) {
	db.DLPrintf("FSCLNT", "unionScan: %v %v\n", dirents, q)
	for _, de := range dirents {
		// Read the link
		_, err := pc.Walk(fid, fid2, []string{de.Name})
		if err != nil {
			return nil, err
		}
		target, err := fsc.readlink(pc, fid2)
		if err != nil {
			return nil, err
		}
		if fsc.unionMatch(q, target) {
			reply, err := pc.Walk(fid, fid2, []string{de.Name})
			if err != nil {
				return nil, err
			}
			return reply, nil
		}
	}
	return nil, nil
}

func (fsc *FsClient) unionLookup(pc *protclnt.ProtClnt, fid, fid2 np.Tfid, q string) (*np.Rwalk, error) {
	db.DLPrintf("FSCLNT", "unionLookup: %v %v %v\n", fid, fid2, q)
	_, err := pc.Open(fid, np.OREAD)
	if err != nil {
		return nil, err
	}
	off := np.Toffset(0)
	for {
		reply, err := pc.Read(fid, off, 1024)
		if err != nil {
			return nil, err
		}
		if len(reply.Data) == 0 {
			return nil, fmt.Errorf("Not found")
		}
		dirents, err := npcodec.Byte2Dir(reply.Data)
		if err != nil {
			return nil, err
		}
		reply1, err := fsc.unionScan(pc, fid, fid2, dirents, q)
		if err != nil {
			return nil, err
		}
		if reply1 != nil {
			return reply1, nil
		}
		off += 1024
	}
}
