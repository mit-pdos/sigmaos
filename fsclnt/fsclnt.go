package fsclnt

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npclnt"
)

const (
	MAXFD      = 20
	MAXSYMLINK = 4
)

type FdState struct {
	offset np.Toffset
	fid    np.Tfid
}

type FsClient struct {
	mu    sync.Mutex
	fds   []FdState
	fids  map[np.Tfid]*Path
	npc   *npclnt.NpClnt
	mount *Mount
	next  np.Tfid
	uname string
}

// XXX need mutex for several threads share FsClient
func MakeFsClient(uname string) *FsClient {
	fsc := &FsClient{}
	fsc.fds = make([]FdState, 0, MAXFD)
	fsc.fids = make(map[np.Tfid]*Path)
	fsc.mount = makeMount()
	fsc.npc = npclnt.MakeNpClnt(false)
	fsc.next = 1
	fsc.uname = uname
	db.SetDebug(false)
	rand.Seed(time.Now().UnixNano())
	return fsc
}

func (fsc *FsClient) String() string {
	str := fmt.Sprintf("Fsclnt table:\n")
	str += fmt.Sprintf("fds %v\n", fsc.fds)
	for k, v := range fsc.fids {
		str += fmt.Sprintf("fid %v chan %v\n", k, v)
	}
	return str
}

func (fsc *FsClient) Uname() string {
	return fsc.uname
}

func (fsc *FsClient) npch(fid np.Tfid) *npclnt.NpChan {
	fsc.mu.Lock()
	defer fsc.mu.Unlock()

	return fsc.fids[fid].npch
}

func (fsc *FsClient) findfd(nfid np.Tfid) int {
	fsc.mu.Lock()
	defer fsc.mu.Unlock()

	for fd, fdst := range fsc.fds {
		if fdst.fid == np.NoFid {
			fsc.fds[fd].offset = 0
			fsc.fds[fd].fid = nfid
			return fd
		}
	}
	// no free one
	fsc.fds = append(fsc.fds, FdState{0, nfid})
	return len(fsc.fds) - 1
}

func (fsc *FsClient) closefd(fd int) {
	fsc.mu.Lock()
	defer fsc.mu.Unlock()

	fsc.fds[fd].fid = np.NoFid
}

func (fsc *FsClient) allocFid() np.Tfid {
	fsc.mu.Lock()
	defer fsc.mu.Unlock()

	fid := fsc.next
	fsc.next += 1
	return fid
}

func (fsc *FsClient) lookup(fd int) (np.Tfid, error) {
	fsc.mu.Lock()
	defer fsc.mu.Unlock()

	if fsc.fds[fd].fid == np.NoFid {
		return np.NoFid, errors.New("Non-existing")
	}
	return fsc.fds[fd].fid, nil
}

func (fsc *FsClient) lookupSt(fd int) (*FdState, error) {
	fsc.mu.Lock()
	defer fsc.mu.Unlock()

	if fd < 0 || fd >= len(fsc.fds) {
		return nil, fmt.Errorf("Too big fd %v", fd)
	}
	if fsc.fds[fd].fid == np.NoFid {
		return nil, fmt.Errorf("Non-existing fd %v", fd)
	}
	return &fsc.fds[fd], nil
}

func (fsc *FsClient) Mount(fid np.Tfid, path string) error {
	_, ok := fsc.fids[fid]
	if !ok {
		return errors.New("Unknown fid")
	}
	db.DPrintf("%v: Mount %v at %v %v\n", fsc.uname, fid, path, fsc.npch(fid))
	fsc.mount.add(np.Split(path), fid)
	return nil
}

func (fsc *FsClient) Close(fd int) error {
	fid, err := fsc.lookup(fd)
	if err != nil {
		return err
	}
	err = fsc.npch(fid).Clunk(fid)
	if err == nil {
		fsc.closefd(fd)
	}
	return err
}

// XXX if server lives in this process, do something special?  FsClient doesn't
// know about the server currently.
func (fsc *FsClient) AttachChannel(fid np.Tfid, server string, p []string) (*Path, error) {
	reply, err := fsc.npc.Attach(server, fsc.Uname(), fid, p)
	if err != nil {
		return nil, err
	}
	ch := fsc.npc.MakeNpChan(server)
	return makePath(ch, p, []np.Tqid{reply.Qid}), nil
}

func (fsc *FsClient) Attach(server string, path string) (np.Tfid, error) {
	p := np.Split(path)
	fid := fsc.allocFid()
	ch, err := fsc.AttachChannel(fid, server, p)
	if err != nil {
		return np.NoFid, err
	}
	fsc.fids[fid] = ch
	db.DPrintf("%v: Attach -> fid %v %v %v\n", fsc.uname, fid, fsc.fids[fid], fsc.fids[fid].npch)
	return fid, nil
}

func (fsc *FsClient) clone(fid np.Tfid) (np.Tfid, error) {
	fid1 := fsc.allocFid()
	_, err := fsc.npch(fid).Walk(fid, fid1, nil)
	if err != nil {
		// XXX free fid
		return np.NoFid, err
	}
	fsc.fids[fid1] = fsc.fids[fid].copyPath()
	return fid1, err
}

func (fsc *FsClient) closeFid(fid np.Tfid) {
	err := fsc.npch(fid).Clunk(fid)
	if err != nil {
		log.Printf("closeFid clunk failed %v\n", err)
	}
	delete(fsc.fids, fid)
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
	defer fsc.closeFid(fid1)

	fid2 := fsc.allocFid()
	reply, err := fsc.npch(fid1).Walk(fid1, fid2, rest)
	if err != nil {
		return np.NoFid, 0, err
	}
	todo := len(rest) - len(reply.Qids)
	db.DPrintf("%v: walkOne rest %v -> %v %v", fsc.uname, rest, reply.Qids, todo)

	fsc.fids[fid2] = fsc.fids[fid1].copyPath()
	fsc.fids[fid2].addn(reply.Qids, rest)
	return fid2, todo, nil
}

func isRemoteTarget(target string) bool {
	return strings.Contains(target, ":")
}

// XXX more robust impl
func splitTarget(target string) (string, string) {
	parts := strings.Split(target, ":")
	server := parts[0] + ":" + parts[1] + ":" + parts[2] + ":" + parts[3]
	return server, parts[len(parts)-1]
}

func (fsc *FsClient) autoMount(target string, path []string) error {
	db.DPrintf("%v: automount %v to %v\n", fsc.uname, target, path)
	server, _ := splitTarget(target)
	fid, err := fsc.Attach(server, "")
	if err != nil {
		log.Fatal("Attach error: ", err)
	}
	return fsc.Mount(fid, np.Join(path))
}

func (fsc *FsClient) walkMany(path []string, resolve bool) (np.Tfid, error) {
	for i := 0; i < MAXSYMLINK; i++ {
		fid, todo, err := fsc.walkOne(path)
		if err != nil {
			return fid, err
		}
		qid := fsc.fids[fid].lastqid()

		// if todo == 0 and !resolve, don't resolve symlinks, so
		// that the client can remove a symlink
		if qid.Type == np.QTSYMLINK && (todo > 0 || (todo == 0 && resolve)) {
			target, err := fsc.Readlink(fid)
			if err != nil {
				return np.NoFid, err
			}
			i := len(path) - todo
			rest := path[i:]
			if isRemoteTarget(target) {
				err = fsc.autoMount(target, path[:i])
				if err != nil {
					return np.NoFid, err
				}
				path = append(path[:i], rest...)
			} else {
				path = append(np.Split(target), rest...)

			}
		} else {
			return fid, err

		}
	}
	return np.NoFid, errors.New("too many iterations")
}

func (fsc *FsClient) Create(path string, perm np.Tperm, mode np.Tmode) (int, error) {
	db.DPrintf("%v: Create %v\n", fsc.uname, path)
	p := np.Split(path)
	dir := p[0 : len(p)-1]
	base := p[len(p)-1]
	fid, err := fsc.walkMany(dir, true)
	if err != nil {
		return -1, err
	}
	reply, err := fsc.npch(fid).Create(fid, base, perm, mode)
	if err != nil {
		return -1, err
	}
	fsc.fids[fid].add(base, reply.Qid)
	fd := fsc.findfd(fid)
	return fd, nil
}

// XXX reduce duplicattion with Create
func (fsc *FsClient) CreateAt(dfd int, name string, perm np.Tperm, mode np.Tmode) (int, error) {
	db.DPrintf("%v: CreateAt %v at %v\n", fsc.uname, name, dfd)
	fid, err := fsc.lookup(dfd)
	if err != nil {
		return -1, err
	}
	fid1, err := fsc.clone(fid)
	if err != nil {
		return -1, err
	}
	reply, err := fsc.npch(fid1).Create(fid1, name, perm, mode)
	if err != nil {
		return -1, err
	}
	fsc.fids[fid1].add(name, reply.Qid)
	fd := fsc.findfd(fid1)
	return fd, nil
}

// XXX move to fslib, perhaps implement as a device, or part of 9P?
func (fsc *FsClient) Pipe(path string, perm np.Tperm) error {
	db.DPrintf("%v: Mkpipe %v\n", fsc.uname, path)
	p := np.Split(path)
	dir := p[0 : len(p)-1]
	base := p[len(p)-1]
	fid, err := fsc.walkMany(dir, true)
	if err != nil {
		return err
	}
	_, err = fsc.npch(fid).Mkpipe(fid, base, perm)
	return err
}

// XXX update pathname associated with fid in Channel
func (fsc *FsClient) Rename(old string, new string) error {
	db.DPrintf("%v: Rename %v %v\n", fsc.uname, old, new)
	fid, err := fsc.walkMany(np.Split(old), np.EndSlash(old))
	if err != nil {
		return err
	}
	fid1, rest := fsc.mount.resolve(np.Split(new))
	if fid1 == np.NoFid {
		return errors.New("Bad destination")

	}
	// XXX check fid is at same server?
	// XXX deal with symbolic names on rest
	st := &np.Stat{}
	st.Name = strings.Join(rest, "/")
	_, err = fsc.npch(fid).Wstat(fid, st)
	return err
}

func (fsc *FsClient) Remove(name string) error {
	db.DPrintf("%v: Remove %v\n", fsc.uname, name)
	fid, err := fsc.walkMany(np.Split(name), np.EndSlash(name))
	if err != nil {
		return err
	}
	err = fsc.npch(fid).Remove(fid)
	return err
}

func (fsc *FsClient) Stat(name string) (*np.Stat, error) {
	db.DPrintf("%v: Stat %v\n", fsc.uname, name)
	fid, err := fsc.walkMany(np.Split(name), true)
	if err != nil {
		return nil, err
	}
	reply, err := fsc.npch(fid).Stat(fid)
	if err != nil {
		return nil, err
	}
	return &reply.Stat, nil
}

// XXX clone fid?
func (fsc *FsClient) Readlink(fid np.Tfid) (string, error) {
	_, err := fsc.npch(fid).Open(fid, np.OREAD)
	if err != nil {
		return "", err
	}
	reply, err := fsc.npch(fid).Read(fid, 0, 1024)
	if err != nil {
		return "", err
	}
	// XXX close fid
	return string(reply.Data), nil
}

func (fsc *FsClient) Open(path string, mode np.Tmode) (int, error) {
	db.DPrintf("%v: Open %v %v\n", fsc.uname, path, mode)
	fid, err := fsc.walkMany(np.Split(path), true)
	if err != nil {
		return -1, err
	}
	_, err = fsc.npch(fid).Open(fid, mode)
	if err != nil {
		return -1, err
	}
	// XXX check reply.Qid?
	fd := fsc.findfd(fid)
	return fd, nil

}

func (fsc *FsClient) OpenAt(dfd int, name string, mode np.Tmode) (int, error) {
	db.DPrintf("%v: OpenAt %v %v %v\n", fsc.uname, dfd, name, mode)

	fid, err := fsc.lookup(dfd)
	if err != nil {
		return -1, err
	}

	fid1, err := fsc.clone(fid)
	if err != nil {
		return -1, err
	}

	n := []string{name}
	reply, err := fsc.npch(fid).Walk(fid, fid1, n)
	if err != nil {
		return -1, err
	}
	fsc.fids[fid1].addn(reply.Qids, n)

	_, err = fsc.npch(fid1).Open(fid1, mode)
	if err != nil {
		return -1, err
	}
	// XXX check reply.Qid?
	fd := fsc.findfd(fid1)
	return fd, nil

}

func (fsc *FsClient) Read(fd int, cnt np.Tsize) ([]byte, error) {
	fdst, err := fsc.lookupSt(fd)
	if err != nil {
		return nil, err
	}
	reply, err := fsc.npch(fdst.fid).Read(fdst.fid, fdst.offset, cnt)
	if err != nil {
		return nil, err
	}
	fdst.offset += np.Toffset(len(reply.Data))
	return reply.Data, err
}

func (fsc *FsClient) Write(fd int, data []byte) (np.Tsize, error) {
	fdst, err := fsc.lookupSt(fd)
	if err != nil {
		return 0, err
	}
	reply, err := fsc.npch(fdst.fid).Write(fdst.fid, fdst.offset, data)
	if err != nil {
		return 0, err
	}
	fdst.offset += np.Toffset(reply.Count)
	return reply.Count, err
}
