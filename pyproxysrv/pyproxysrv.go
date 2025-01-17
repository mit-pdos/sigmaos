package pyproxysrv

import (
	"bufio"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

// PyProxySrv maintains the state of the pyproxysrv.
type PyProxySrv struct {
	pe *proc.ProcEnv
	sc *sigmaclnt.SigmaClnt
}

func NewPyProxySrv(pe *proc.ProcEnv) (*PyProxySrv, error) {
	// Create the proxy socket
	socket, err := net.Listen("unix", sp.SIGMA_PYPROXY_SOCKET)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(sp.SIGMA_PYPROXY_SOCKET, 0777); err != nil {
		db.DFatalf("Err chmod sigmasocket: %v", err)
	}
	db.DPrintf(db.TEST, "runServer: pyproxysrv listening on %v", sp.SIGMA_PYPROXY_SOCKET)

	pps := &PyProxySrv{
		pe: pe,
	}
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		return nil, err
	}
	pps.sc = sc

	go pps.runServer(socket)

	return pps, nil
}

func (pps *PyProxySrv) Shutdown() {
	db.DPrintf(db.PYPROXYSRV, "Shutdown")
	os.Remove(sp.SIGMA_PYPROXY_SOCKET)
}

// Recursively copies all the contents of currPath to destPath. Does
// not yet handle concurrent modifications of the source directory.
func (pps *PyProxySrv) copyLib(currPath string, destPath string) error {
	db.DPrintf(db.PYPROXYSRV, "Copying %v to %v", currPath, destPath)
	isDir, err := pps.sc.IsDir(currPath)
	if err != nil {
		return err
	}

	if !isDir {
		// Copy the file over
		srcFile, err := pps.sc.GetFile(currPath)
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(destPath, srcFile, 0777)
		if err != nil {
			return err
		}

		return nil
	}

	os.Mkdir(destPath, 0777)

	_, err = pps.sc.ProcessDir(currPath, func(st *sp.Tstat) (bool, error) {
		newSrc := filepath.Join(currPath, st.Name)
		newDst := filepath.Join(destPath, st.Name)
		return false, pps.copyLib(newSrc, newDst)
	})
	if err != nil {
		return err
	}

	return nil
}

func (pps *PyProxySrv) fetchLib(libName string) {
	db.DPrintf(db.PYPROXYSRV, "Fetching %v", libName)
	pn := filepath.Join(sp.NAMED, "s3", "~any", "ivy-tutorial-test", libName)
	libDest := filepath.Join("/tmp", "python", "Lib", libName)

	err := pps.copyLib(pn, libDest)
	if err != nil {
		db.DPrintf(db.PYPROXYSRV_ERR, "copyLib: %v", err)
		return
	}

	return
}

func (pps *PyProxySrv) handleNewConn(conn *net.UnixConn) {
	reader := bufio.NewReader(conn)
	libContents := make(map[string]bool)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			db.DPrintf(db.PYPROXYSRV_ERR, "reader: rf err %v\n", err)
			return
		}

		db.DPrintf(db.PYPROXYSRV, "reader: received %v", line)

		reqPrefix := line[:2]
		reqPath := line[2:]

		if reqPrefix == "pb" {
			// Initialization
			db.DPrintf(db.PYPROXYSRV, "reader: received initialization request\n", err)

			// Record contents at the Lib directory
			libFiles, err := os.ReadDir("/tmp/python/Lib")
			if err != nil {
				db.DPrintf(db.PYPROXYSRV_ERR, "reader: err reading Python Lib %v\n", err)
				return
			}

			for _, file := range libFiles {
				db.DPrintf(db.PYPROXYSRV, "reader: Lib: %v\n", file.Name())
				libContents[file.Name()] = true
			}
		} else if reqPrefix == "pf" {
			// Searching for Python file
			if strings.HasPrefix(reqPath, "/Lib") {
				// Check that the requested library exists
				libName := strings.TrimSpace(strings.Split(reqPath, "/")[2])
				if libContents[libName] == false {
					// Read all library contents from S3 into local
					pps.fetchLib(libName)
					libContents[libName] = true
				}
			}
		}

		response := []byte("d")
		_, err = conn.Write(response)
		if err != nil {
			db.DPrintf(db.PYPROXYSRV_ERR, "reader: wf err %v\n", err)
			return
		}
	}
}

func (pps *PyProxySrv) runServer(l net.Listener) {
	db.DPrintf(db.PYPROXYSRV, "pyproxysrv running")
	for {
		conn, err := l.Accept()
		if err != nil {
			db.DFatalf("Error pyproxysrv Accept: %v", err)
			return
		}
		// Handle incoming connection
		go pps.handleNewConn(conn.(*net.UnixConn))
	}
}
