package name

import (
	"errors"
	"log"
	"path/filepath"
	"strings"

	"ulambda/fsrpc"
)

// Base("/") is "/", so check for "/" too. Base(".") is "." and Dir(".") is
// "." too
func IsCurrentDir(name string) bool {
	return name == "." || name == "/" || name == ""
}

type Root struct {
	root    *fsrpc.Ufid
	dir     *Dir
	inodes  map[fsrpc.Fid]*Inode
	rFid    fsrpc.Fid
	nextFid fsrpc.Fid
}

func MakeRoot(root *fsrpc.Ufid) *Root {
	r := Root{}
	r.dir = makeDir()
	r.root = root
	r.inodes = make(map[fsrpc.Fid]*Inode)
	r.rFid = fsrpc.RootFid()
	r.nextFid = fsrpc.MakeFid(0, r.rFid.Id+1)
	r.inodes[r.rFid] = makeInode(DirT, r.rFid, r.dir)
	return &r
}

func (root *Root) Myroot() *fsrpc.Ufid {
	return root.root
}

// XXX bump version # if allocating same inode #
// XXX a better inum allocation plan
func (root *Root) allocInum() fsrpc.Fid {
	fid := root.nextFid
	root.nextFid.Id += 1
	return fid
}

func (root *Root) makeUfid(fid fsrpc.Fid) *fsrpc.Ufid {
	ufid := *root.root
	ufid.Fid = fid
	return &ufid
}

func (root *Root) RootFid() fsrpc.Fid {
	return root.rFid
}

func (root *Root) Fid2Inode(fid fsrpc.Fid) (*Inode, error) {
	if inode, ok := root.inodes[fid]; ok {
		return inode, nil
	} else {
		return nil, errors.New("Unknown fid")
	}
}

func (root *Root) fid2Dir(fid fsrpc.Fid) (*Dir, error) {
	inode, err := root.Fid2Inode(fid)
	if err != nil {
		return nil, err
	}
	if inode.Type == DirT {
		return inode.Data.(*Dir), nil
	} else {
		return nil, errors.New("Not a directory")
	}
}

func (root *Root) Namei(start fsrpc.Fid, path []string) (*Inode, []string, error) {
	if IsCurrentDir(path[0]) {
		i, err := root.Fid2Inode(start)
		return i, path[1:], err
	}
	dir, err := root.fid2Dir(start)
	if err != nil {
		return nil, nil, err
	}
	return dir.Namei(path)
}

func (root *Root) Walk(start fsrpc.Fid, path string) (*fsrpc.Ufid, string, error) {
	log.Printf("Walk %v\n", path)
	inode, rest, err := root.Namei(start, strings.Split(path, "/"))
	if err == nil {
		if inode.Type == MountT {
			ufid := inode.Data.(*fsrpc.Ufid)
			return ufid, strings.Join(rest, "/"), err
		} else {
			return root.makeUfid(inode.Fid), strings.Join(rest, "/"), err
		}
	} else {
		return nil, "", err
	}
}

func (root *Root) Ls(fid fsrpc.Fid, n int) (string, error) {
	dir, err := root.fid2Dir(fid)
	if err != nil {
		return "", err
	}
	return dir.ls(n), nil
}

func (root *Root) Open(start fsrpc.Fid, path string) (*Inode, error) {
	dirp := filepath.Dir(path)
	base := filepath.Base(path)
	log.Printf("Open %v %v(%v,%v)\n", start, path, dirp, base)
	if inode, _, err := root.Namei(start, strings.Split(dirp, "/")); err == nil {
		inode, err = inode.lookup(base)
		if err != nil {
			return inode, err
		}
		root.inodes[inode.Fid] = inode
		return inode, nil
	} else {
		return nil, err
	}
}

func (root *Root) Create(start fsrpc.Fid, path string) (*Inode, error) {
	dirp := filepath.Dir(path)
	base := filepath.Base(path)
	log.Printf("Create %v %v(%v,%v)\n", path, start, dirp, base)
	if inode, _, err := root.Namei(start, strings.Split(dirp, "/")); err == nil {
		inode, err = inode.create(root, FileT, base, []byte{})
		if err != nil {
			return inode, err
		}
		root.inodes[inode.Fid] = inode
		return inode, nil
	} else {
		return nil, err
	}
}

func (root *Root) MkDir(start fsrpc.Fid, path string) (*Inode, error) {
	dirp := filepath.Dir(path)
	base := filepath.Base(path)
	log.Printf("Mkdir %v %v(%v,%v)\n", path, start, dirp, base)
	if inode, _, err := root.Namei(start, strings.Split(dirp, "/")); err == nil {
		inode, err = inode.create(root, DirT, base, makeDir())
		if err != nil {
			return inode, err
		}
		root.inodes[inode.Fid] = inode
		return inode, nil
	} else {
		return nil, err
	}
}

func (root *Root) Mount(ufid *fsrpc.Ufid, start fsrpc.Fid, path string) error {
	dirp := filepath.Dir(path)
	base := filepath.Base(path)
	log.Printf("Mount %v at (%v, %v,%v)\n", ufid, start, dirp, base)
	if inode, _, err := root.Namei(start, strings.Split(dirp, "/")); err == nil {
		inode, err = inode.create(root, MountT, base, ufid)
		if err != nil {
			return err
		}
		root.inodes[inode.Fid] = inode
		return nil
	} else {
		return err
	}
}

func (root *Root) MkNod(start fsrpc.Fid, path string, rw Dev) error {
	dirp := filepath.Dir(path)
	base := filepath.Base(path)
	log.Printf("Mknod %v %v(%v,%v)\n", path, start, dirp, base)
	if inode, _, err := root.Namei(start, strings.Split(dirp, "/")); err == nil {
		inode, err = inode.create(root, DevT, base, rw)
		if err != nil {
			return err
		}
		root.inodes[inode.Fid] = inode
		return nil
	} else {
		return err
	}
}

func (root *Root) Pipe(fid fsrpc.Fid, path string) error {
	return nil
}

func (root *Root) Write(fid fsrpc.Fid, data []byte) (int, error) {
	log.Printf("Write %v\n", fid, data)
	inode, err := root.Fid2Inode(fid)
	if err != nil {
		return 0, err
	}
	if inode.Type == DevT {
		dev := inode.Data.(Dev)
		return dev.Write(fid, data)
	} else {
		inode.Data = data
		return len(data), nil
	}
}

func (root *Root) Read(fid fsrpc.Fid, n int) ([]byte, error) {
	log.Printf("Read %v\n", fid)
	inode, err := root.Fid2Inode(fid)
	if err != nil {
		return nil, err
	}
	if inode.Type == DevT {
		dev := inode.Data.(Dev)
		return dev.Read(fid, n)
	} else {
		log.Printf("-> %v\n", inode.Data.([]byte))
		return inode.Data.([]byte), nil
	}
}
