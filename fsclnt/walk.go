package fsclnt

import (
	"errors"
	"fmt"
	"log"
	"strings"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npclnt"
	"ulambda/npcodec"
)

const (
	MAXSYMLINK = 4
)

func (fsc *FsClient) walkMany(path []string, resolve bool) (np.Tfid, error) {
	for i := 0; i < MAXSYMLINK; i++ {
		fid, todo, err := fsc.walkOne(path)
		if err != nil {
			return fid, err
		}
		qid := fsc.fids[fid].lastqid()

		// if todo == 0 and !resolve, don't resolve symlinks, so
		// that the client can remove a symlink
		if qid.Type == np.QTSYMLINK && (todo > 0 ||
			(todo == 0 && resolve)) {
			path, err = fsc.walkSymlink(fid, path, todo)
			if err != nil {
				return np.NoFid, err
			}
		} else {
			return fid, err
		}
	}
	return np.NoFid, errors.New("too many iterations")
}

func (fsc *FsClient) walkOne(path []string) (np.Tfid, int, error) {
	fid, rest := fsc.mount.resolve(path)
	db.DPrintf("%v: walkOne: mount -> %v %v\n", fsc.uname, fid, rest)
	if fid == np.NoFid {
		return np.NoFid, 0, errors.New("Unknown file")

	}
	fid1, err := fsc.clone(fid)
	if err != nil {
		return np.NoFid, 0, err
	}
	defer fsc.clunkFid(fid1)
	fid2 := fsc.allocFid()
	first, union := IsUnion(rest)
	var reply *np.Rwalk
	todo := 0
	if union {
		reply, err = fsc.walkUnion(fsc.npch(fid1), fid1, fid2,
			first, rest[len(first)])
		rest = rest[len(first)+1:]
		todo = len(rest)
	} else {
		reply, err = fsc.npch(fid1).Walk(fid1, fid2, rest)
		if err != nil {
			return np.NoFid, 0, err
		}
		todo = len(rest) - len(reply.Qids)
		db.DPrintf("%v: walkOne rest %v -> %v %v", fsc.uname, rest,
			reply.Qids, todo)
	}
	fsc.addFid(fid2, fsc.path(fid1).copyPath())
	fsc.path(fid2).addn(reply.Qids, rest)
	return fid2, todo, nil
}

func (fsc *FsClient) walkSymlink(fid np.Tfid, path []string, todo int) ([]string, error) {
	target, err := fsc.Readlink(fid)
	if err != nil {
		return nil, err
	}
	i := len(path) - todo
	rest := path[i:]
	if IsRemoteTarget(target) {
		err = fsc.autoMount(target, path[:i])
		if err != nil {
			return nil, err
		}
		path = append(path[:i], rest...)
	} else {
		path = append(np.Split(target), rest...)
	}
	return path, nil
}

func IsRemoteTarget(target string) bool {
	return strings.Contains(target, ":")
}

func SplitTarget(target string) string {
	parts := strings.Split(target, ":")
	var server string

	if strings.HasPrefix(parts[0], "[") { // IPv6: [::]:port:pubkey:name
		server = parts[0] + ":" + parts[1] + ":" + parts[2] + ":" + parts[3]
	} else { // IPv4
		server = parts[0] + ":" + parts[1]
	}
	return server
}

func (fsc *FsClient) autoMount(target string, path []string) error {
	db.DPrintf("%v: automount %v to %v\n", fsc.uname, target, path)
	server := SplitTarget(target)
	fid, err := fsc.Attach(server, "")
	if err != nil {
		log.Fatal("Attach error: ", err)
	}
	return fsc.Mount(fid, np.Join(path))
}

func IsUnion(path []string) ([]string, bool) {
	for i, c := range path {
		if strings.HasPrefix(c, "~") {
			return path[:i], true
		}
	}
	return nil, false
}

func (fsc *FsClient) walkUnion(ch *npclnt.NpChan, fid, fid2 np.Tfid, dir []string, q string) (*np.Rwalk, error) {
	db.DPrintf("Walk union: %v %v\n", dir, q)
	fid3 := fsc.allocFid()
	reply, err := ch.Walk(fid, fid3, dir)
	if err != nil {
		return nil, err
	}
	reply, err = fsc.unionLookup(fsc.npch(fid), fid3, fid2, q)
	if err != nil {
		return nil, err
	}
	db.DPrintf("%v: unionLookup -> %v %v", fsc.uname, fid2, reply.Qids)
	return reply, nil
}

func (fsc *FsClient) unionMatch(q, name string) bool {
	db.DPrintf("unionMatch %v %v\n", q, name)
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
			db.DPrintf("match %v\n", parts[0])
			return true
		}
		return false
	default:
		return true
	}
	return true
}

func (fsc *FsClient) unionScan(ch *npclnt.NpChan, fid, fid2 np.Tfid, dirents []*np.Stat, q string) (*np.Rwalk, error) {
	db.DPrintf("unionScan: %v %v\n", dirents, q)
	for _, de := range dirents {
		if fsc.unionMatch(q, de.Name) {
			reply, err := ch.Walk(fid, fid2, []string{de.Name})
			if err != nil {
				return nil, err
			}
			return reply, nil
		}
	}
	return nil, nil
}

func (fsc *FsClient) unionLookup(ch *npclnt.NpChan, fid, fid2 np.Tfid, q string) (*np.Rwalk, error) {
	db.DPrintf("%v: unionLookup: %v %v %v\n", fsc.uname, fid, fid2, q)
	_, err := ch.Open(fid, np.OREAD)
	if err != nil {
		return nil, err
	}
	off := np.Toffset(0)
	for {
		reply, err := ch.Read(fid, off, 1024)
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
		reply1, err := fsc.unionScan(ch, fid, fid2, dirents, q)
		if err != nil {
			return nil, err
		}
		if reply1 != nil {
			return reply1, nil
		}
		off += 1024
	}
}
