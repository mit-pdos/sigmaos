package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	db "sigmaos/debug"

	"time"
)

//
// sudo criu dump -D dump -t 1371435 --shell-job
// sudo criu restore --images-dir dump --shell-job
//

type HelloRoot struct {
	fs.Inode
}

func (r *HelloRoot) OnAdd(ctx context.Context) {
	ch := r.NewPersistentInode(
		ctx, &fs.MemRegularFile{
			Data: []byte("file.txt"),
			Attr: fuse.Attr{
				Mode: 0644,
			},
		}, fs.StableAttr{Ino: 2})
	r.AddChild("file.txt", ch, false)
}

func (r *HelloRoot) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	return 0
}

var _ = (fs.NodeGetattrer)((*HelloRoot)(nil))
var _ = (fs.NodeOnAdder)((*HelloRoot)(nil))

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v <sleep_length>\n", os.Args[0])
		os.Exit(1)
	}

	mnt := "/mnt/binfs"

	db.DPrintf(db.ALWAYS, "Pid: %d", os.Getpid())

	n, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Printf("Atoi err %v\n", err)
		return
	}
	timer := time.NewTicker(time.Duration(n) * time.Second)

	f, err := os.Create("/tmp/ckptsrv.txt")
	if err != nil {
		db.DFatalf("Error creating %v\n", err)
	}

	var server *fuse.Server
	if true {
		orig := "/tmp/fuse"
		loopbackRoot, err := fs.NewLoopbackRoot(orig)
		if err != nil {
			log.Fatalf("NewLoopbackRoot(%s): %v\n", orig, err)
		}

		sec := time.Second
		opts := &fs.Options{
			// The timeout options are to be compatible with libfuse defaults,
			// making benchmarking easier.
			AttrTimeout:  &sec,
			EntryTimeout: &sec,

			NullPermissions: true, // Leave file permissions on "000" files as-is

			MountOptions: fuse.MountOptions{
				AllowOther:        false,
				Debug:             false,
				DirectMount:       false,
				DirectMountStrict: false,
				FsName:            orig,       // First column in "df -T": original dir
				Name:              "loopback", // Second column in "df -T" will be shown as "fuse." + Name
			},
		}
		if opts.AllowOther {
			// Make the kernel check file permissions for us
			opts.MountOptions.Options = append(opts.MountOptions.Options, "default_permissions")
		}
		q := false
		quiet := &q

		// Enable diagnostics logging
		if !*quiet {
			opts.Logger = log.New(os.Stderr, "", 0)
		}
		server, err = fs.Mount(mnt, loopbackRoot, opts)
		if err != nil {
			log.Fatalf("Mount fail: %v\n", err)
		}
		if !*quiet {
			fmt.Println("Mounted!")
		}
	} else {
		opts := &fs.Options{}
		opts.Debug = false
		server, err = fs.Mount(mnt, &HelloRoot{}, opts)
		if err != nil {
			db.DFatalf("Mount fail: %v\n", err)
		}
	}

	// listOpenfiles()

	for {
		select {
		case <-timer.C:
			fmt.Println("!")
			f.Write([]byte("exiting"))
			break
			// return
		default:
			fmt.Print(".")
			f.Write([]byte("."))
			time.Sleep(2 * time.Second)
		}
	}
	server.Wait()
	server.Unmount()
}

func listOpenfiles() {
	files, _ := ioutil.ReadDir("/proc")
	fmt.Println("listOpenfiles:")
	for _, f := range files {
		m, _ := filepath.Match("[0-9]*", f.Name())
		if f.IsDir() && m {
			fdpath := filepath.Join("/proc", f.Name(), "fd")
			ffiles, _ := ioutil.ReadDir(fdpath)
			for _, f := range ffiles {
				fpath, err := os.Readlink(filepath.Join(fdpath, f.Name()))
				if err != nil {
					fmt.Printf("listOpenfiles %v: err %v\n", f.Name(), err)
					continue
				}
				fmt.Printf("%v : %v\n", f, fpath)
			}
		}
	}
}
