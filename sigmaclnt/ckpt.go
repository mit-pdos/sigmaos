package sigmaclnt

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	db "sigmaos/debug"
	dialproxytrans "sigmaos/dialproxy/transport"
	"sigmaos/frame"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

// CheckpointMe makes a checkpoint of the calling proc and terminates
// the proc.  The parent can spawn a new proc from the checkpoint; the
// new proc resumes in CheckpointMe and return a new SigmaClnt to the
// new proc.
func (sc *SigmaClnt) CheckpointMe(ckptpn string) (*SigmaClnt, error) {
	os.Stdin.Close() // XXX close in StartUproc?

	listOpenfiles()

	// The reader side of the unnamed socketpair created by
	// StartUproc. XXX maybe get the rdr and store it in sc in the
	// initial NewSigmaClnt call
	rdr := os.NewFile(3, "rdr")

	// Set up TCP conn for dump dir
	if _, err := sc.Stat(sp.UX + "~any/"); err != nil {
		db.DFatalf("Stat err %v\n", err)
	}

	// Criu cannot dump named streaming unix sockets, so close the
	// connection with spproxyd.sock.  XXX pass this fd through
	// environment.
	syscall.Close(4)
	syscall.Close(5)

	if err := sc.ProcAPI.(*procclnt.ProcClnt).CheckpointMe(ckptpn); err != nil && serr.IsErrCode(err, serr.TErrUnreachable) {
		db.DPrintf(db.CKPT, "CheckpointMe err %v\n", err)

		// XXX check that is unreachle error?

		// Close old sigmaclnt
		sc.Close()

		if _, err := rdr.Write([]byte("r")); err != nil {
			db.DFatalf("Write run err %v\n", err)
		}

		// Receive the spproxyd conn from RestoreProc()
		conn, err := receiveConn(rdr)
		if err != nil {
			db.DFatalf("receiveConn err %v\n", err)
		}

		// Receive the new procenv from RestoreProc()
		if err := receiveProcEnv(rdr); err != nil {
			db.DFatalf("receiveProcEnv pe %v err %v\n", err)
		}

		db.DPrintf(db.CKPT, "conn %v ProcEnv %v", conn, proc.GetProcEnv())

		// Initialize new sigmaclnt
		sc, err = NewSigmaClnt(proc.GetProcEnv())
		if err != nil {
			return nil, err
		}
		return sc, nil
	} else {
		return nil, fmt.Errorf("Checkpoint failed")
	}
}

func receiveConn(rdr *os.File) (net.Conn, error) {
	conn, err := net.FileConn(rdr)
	if err != nil {
		return nil, err
	}
	uconn, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, fmt.Errorf("not a unix conn")
	}
	c, err := rcvConn(uconn)
	if err != nil {
		return nil, err
	}
	return c, err
}

func rcvConn(uconn *net.UnixConn) (net.Conn, error) {
	var (
		b   [32]byte
		oob [32]byte
	)
	_, oobn, _, _, err := uconn.ReadMsgUnix(b[:], oob[:])
	if err != nil {
		return nil, err
	}
	messages, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return nil, err
	}
	if len(messages) != 1 {
		return nil, fmt.Errorf("expect 1 message, got %#v", messages)
	}
	message := messages[0]
	fds, err := syscall.ParseUnixRights(&message)
	if err != nil {
		return nil, err
	}
	if len(fds) != 1 {
		return nil, fmt.Errorf("expect 1 fd, got %#v", fds)
	}
	db.DPrintf(db.CKPT, "spproxyd fd %d\n", fds[0])

	os.Setenv(dialproxytrans.SIGMA_DIALPROXY_FD, strconv.Itoa(fds[0]))

	f := os.NewFile(uintptr(fds[0]), "spproxyd")
	conn, err := net.FileConn(f)
	if err != nil {
		return nil, fmt.Errorf("FileConn %v err %v", fds[0], err)
	}
	return conn, nil
}

func receiveProcEnv(rdr *os.File) error {
	conn, err := net.FileConn(rdr)
	if err != nil {
		return err
	}
	uconn, ok := conn.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("not a unix conn")
	}
	b, sr := frame.ReadFrame(uconn)
	if sr != nil {
		return err
	}
	proc.SetProcEnv(string(b))
	return nil
}

func infoFd(fd int) {
	sotype, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_TYPE)
	if err != nil {
		db.DPrintf(db.CKPT, "GetsockoptInt %d %v\n", fd, err)
	}
	lsa, err := syscall.Getsockname(fd)
	db.DPrintf(db.CKPT, "sock %v %v %v\n", sotype, lsa, err)
}

// For debugging
func listDir(dir string) {
	files, _ := ioutil.ReadDir(dir)
	fmt.Print("listDir:[")
	for _, f := range files {
		fmt.Printf("%v,", f.Name())
	}
	fmt.Println("]")
}

// For debugging
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
				fmt.Printf("%v: %v : %v\n", f.Name(), f, fpath)
			}
		}
	}
}
