package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	db "sigmaos/debug"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v <child>\n", os.Args[0])
		os.Exit(1)
	}

	if os.Args[1] == "child" {
		out, err := os.Create("/tmp/log-proc.txt")
		if err != nil {
			db.DFatalf("Error creating %v\n", err)
		}

		rdr := os.NewFile(3, "rdr")
		//db.DPrintf(db.ALWAYS, "Child Pid: %d", os.Getpid())
		b := make([]byte, 100)
		for true {
			_, err := recvmsg(rdr, b)
			out.Write([]byte(fmt.Sprintf("recv msg %s\n", string(b))))
			if err != nil {
				db.DFatalf("Error recvmg %v\n", err)
			}
			time.Sleep(1 * time.Second)
			// db.DPrintf(db.ALWAYS, "recvmsg %d: %s\n", n, string(b))
		}
	}

	rdr, wrt, err := unixPair()
	if err != nil {
		db.DFatalf("Error unixPair %v\n", err)
	}
	child := exec.Command("./bin/linux/ckptsockpair", []string{"child"}...)
	//cmd.Stdin = os.Stdin
	//cmd.Stdout = os.Stdout
	//cmd.Stderr = os.Stderr
	child.ExtraFiles = []*os.File{rdr}
	if err := child.Start(); err != nil {
		db.DFatalf("Error start %v\n", err)
	}
	pid := child.Process.Pid
	ino := ino(rdr)
	rdr.Close()
	if _, err := sendmsg(wrt, []byte("hello")); err != nil {
		db.DFatalf("Error sendmsg %v\n", err)
	}

	ext := "unix[" + strconv.FormatInt(int64(ino), 10) + "]"
	cmd := exec.Command("criu", []string{"dump", "-vvvv", "--shell-job", "-D", "dump", "--log-file", "dump.txt", "--external", ext, "-t", strconv.Itoa(pid)}...)
	if err := cmd.Run(); err != nil {
		db.DFatalf("Error Command dump %v\n", err)
	}
	db.DPrintf(db.ALWAYS, "dump err %v\n", err)

	// Collect child's status after dumping
	if err := child.Wait(); err != nil {
		db.DPrintf(db.ALWAYS, "Error Wait %v\n", err)
	}
	wrt.Close()
	db.DPrintf(db.ALWAYS, "Dumped and terminated child")

	rdr, wrt, err = unixPair()
	if err != nil {
		db.DFatalf("Error unixPair %v\n", err)
	}

	time.Sleep(1 * time.Second)

	db.DPrintf(db.ALWAYS, "Restore child")

	go func() {
		fd := int(rdr.Fd())
		inh := "fd[" + strconv.Itoa(fd) + "]:socket:[" + strconv.FormatInt(int64(ino), 10) + "]"
		cmd := exec.Command("criu", []string{"restore", "-vvvv", "-D", "dump", "--log-file", "restore.txt", "--inherit-fd", inh, "-t", strconv.Itoa(pid)}...)
		if err := cmd.Run(); err != nil {
			db.DFatalf("Error Command restore %v\n", err)
		}
		db.DPrintf(db.ALWAYS, "restore inh %v err %v\n", inh, err)
	}()

	// rdr.Close()

	if _, err := sendmsg(wrt, []byte("bye")); err != nil {
		db.DFatalf("Error sendmsg %v\n", err)
	}

	time.Sleep(1 * time.Second)
}

func recvmsg(f *os.File, b []byte) (int, error) {
	conn, err := net.FileConn(f)
	if err != nil {
		return 0, err
	}
	uconn, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, fmt.Errorf("not a unix conn")
	}
	n, err := uconn.Read(b)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func sendmsg(f *os.File, b []byte) (int, error) {
	conn, err := net.FileConn(f)
	if err != nil {
		return 0, err
	}
	uconn, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, fmt.Errorf("not a unix conn")
	}
	n, err := uconn.Write(b)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func ino(f *os.File) uint64 {
	st := syscall.Stat_t{}
	fd := int(f.Fd())
	if err := syscall.Fstat(fd, &st); err != nil {
		db.DFatalf("fstat %v\n", err)
	}
	db.DPrintf(db.ALWAYS, "fd %v ino %d\n", fd, st.Ino)
	return st.Ino
}

func unixPair() (*os.File, *os.File, error) {
	fd, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return nil, nil, err
	}
	src := os.NewFile(uintptr(fd[0]), "src")
	dst := os.NewFile(uintptr(fd[1]), "dst")
	return src, dst, nil
}
