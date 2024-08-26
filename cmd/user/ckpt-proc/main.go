package main

import (
	"fmt"
	"io/ioutil"
	"math/rand/v2"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	db "sigmaos/debug"
	"sigmaos/netproxytrans"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"

	"time"
)

func main() {
	if len(os.Args) < 5 {
		fmt.Fprintf(os.Stderr, "Usage: %v <no/ext/self> <sleep_length> <npages> <ckpt-pn>\n", os.Args[0])
		os.Exit(1)
	}
	cmd := os.Args[1]
	sec, err := strconv.Atoi(os.Args[2])
	if err != nil {
		db.DFatalf("Atoi error %v\n", err)
		return
	}
	npages, err := strconv.Atoi(os.Args[3])
	if err != nil {
		db.DFatalf("Atoi error %v\n", err)
		return
	}
	ckptpn := os.Args[4]

	db.DPrintf(db.ALWAYS, "Running %v %d %d %v", cmd, sec, npages, ckptpn)

	listOpenfiles()

	var sc *sigmaclnt.SigmaClnt
	if cmd == "no" || cmd == "self" {
		sc, err = sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
		if err != nil {
			db.DFatalf("NewSigmaClnt error %v\n", err)
		}
		err = sc.Started()
		if err != nil {
			db.DFatalf("Started error %v\n", err)
		}
	}

	var rdr *os.File
	if cmd == "self" {
		st := syscall.Stat_t{}
		rdr = os.NewFile(3, "rdr")
		syscall.Fstat(3, &st)
		db.DPrintf(db.ALWAYS, "rdr Ino %v\n", st.Ino)
	}

	timer := time.NewTicker(time.Duration(sec) * time.Second)

	os.Stdin.Close() // XXX close in StartUproc

	if cmd == "ext" {
		syscall.Close(3) // close spproxyd.sock
	}

	f, err := os.Create("/tmp/sigmaos-perf/log.txt")
	if err != nil {
		db.DFatalf("Error creating %v\n", err)
	}

	pagesz := os.Getpagesize()
	mem := make([]byte, pagesz*npages)
	for i := 0; i < npages; i++ {
		mem[i*pagesz] = byte(i)
	}

	f.Write([]byte("."))

	if cmd == "self" {
		_, err := sc.Stat(sp.UX + "~any/")
		if err != nil {
			db.DFatalf("Stat err %v\n", err)
		}

		//syscall.Close(3) // TCP connection
		//syscall.Close(4) // close rpcclnt w. spproxyd.sock?
		syscall.Close(5) // close rpcclnt w. spproxyd.sock?

		if err := sc.CheckpointMe(ckptpn); err != nil {
			db.DPrintf(db.ALWAYS, "CheckpointMe err %v\n", err)

			// listDir("/tmp")
			//listOpenfiles()

			//infoFd(4)
			//infoFd(5)

			sc.Close()

			conn, err := receiveConn(rdr)
			if err != nil {
				db.DFatalf("Restore err %v\n", err)
			}

			db.DPrintf(db.ALWAYS, "ReceiveFd %v", conn)

			sc, err = sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
			if err != nil {
				db.DFatalf("NewSigmaClnt error %v\n", err)
			}

			db.DPrintf(db.ALWAYS, "Mark started")

			err = sc.Started()
			if err != nil {
				db.DFatalf("Started error %v\n", err)
			}

			f.Write([]byte("............exit"))

			db.DPrintf(db.ALWAYS, "ClntExit")

			sc.ClntExitOK()
			os.Exit(1)
		}
	}

	for {
		select {
		case <-timer.C:
			f.Write([]byte("exit"))
			if cmd == "self" {
				sc.ClntExitOK()
			}
			return
		default:
			f.Write([]byte("."))
			r := rand.IntN(npages)
			mem[r*pagesz] = byte(r)
			time.Sleep(1 * time.Second)
		}
	}
}

func listDir(dir string) {
	files, _ := ioutil.ReadDir(dir)
	fmt.Print("listDir:[")
	for _, f := range files {
		fmt.Printf("%v,", f.Name())
	}
	fmt.Println("]")
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
				fmt.Printf("%v: %v : %v\n", f.Name(), f, fpath)
			}
		}
	}
}

func infoFd(fd int) {
	sotype, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_TYPE)
	if err != nil {
		db.DPrintf(db.ALWAYS, "GetsockoptInt %d %v\n", fd, err)
	}
	lsa, err := syscall.Getsockname(fd)
	db.DPrintf(db.ALWAYS, "sock %v %v %v\n", sotype, lsa, err)
}

func receiveConn(dst *os.File) (net.Conn, error) {
	conn, err := net.FileConn(dst)
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
	db.DPrintf(db.ALWAYS, "spproxyd fd %d\n", fds[0])

	os.Setenv(netproxytrans.SIGMA_NETPROXY_FD, strconv.Itoa(fds[0]))

	f := os.NewFile(uintptr(fds[0]), "spproxyd")
	conn, err := net.FileConn(f)
	if err != nil {
		return nil, fmt.Errorf("FileConn %v err %v", fds[0], err)
	}
	return conn, nil
}
