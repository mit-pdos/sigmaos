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

	db.DPrintf(db.PYPROXYSRV, "Finished fetching %v", libName)

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

			// Set up superlib dummy directory
			superlibPath := "/tmp/python/superlib"
			err := os.MkdirAll(superlibPath, 0777)
			if err != nil {
				db.DPrintf(db.PYPROXYSRV_ERR, "reader: err creating superlib %v\n", err)
			}

			// Record contents at the Lib directory
			libFiles, err := os.ReadDir("/tmp/python/Lib")
			if err != nil {
				db.DPrintf(db.PYPROXYSRV_ERR, "reader: err reading Python Lib %v\n", err)
				return
			}

			for _, file := range libFiles {
				db.DPrintf(db.PYPROXYSRV, "reader: Lib: %v\n", file.Name())
				err = os.MkdirAll(filepath.Join(superlibPath, file.Name()), 0777)
				libContents[file.Name()] = true
			}

			// Add all libraries in the S3 bucket to superlib
			pn := filepath.Join(sp.NAMED, "s3", "~any", "ivy-tutorial-test")
			_, err = pps.sc.ProcessDir(pn, func(st *sp.Tstat) (bool, error) {
				err := os.MkdirAll(filepath.Join(superlibPath, st.Name), 0777)
				if err != nil {
					return false, err
				}
				return false, nil
			})
			if err != nil {
				db.DPrintf(db.PYPROXYSRV_ERR, "reader: err adding S3 libs %v", err)
				return
			}
		} else if reqPrefix == "pa" {
			db.DPrintf(db.PYPROXYSRV, "reader: received API request: %v", reqPath)
			db.DPrintf(db.PYPROXYSRV, "reader: proc settings: %v %v %v\n", pps.pe.KernelID, pps.pe.Program, pps.pe.ProcdPIDStr)
			db.DPrintf(db.PYPROXYSRV, "reader: new proc settings: %v %v %v\n", proc.GetProcEnv().KernelID, proc.GetProcEnv().Program, proc.GetProcEnv().PidStr)
			db.DPrintf(db.PYPROXYSRV, "reader: other proc: %v %v %v\n", pps.sc.ProcEnv().KernelID, pps.sc.ProcEnv().Program, pps.sc.ProcEnv().PidStr)
			if strings.HasPrefix(reqPath, "/Started") {
				db.DPrintf(db.PYPROXYSRV, "reader: Started called")
				err := pps.sc.ProcAPI.Started()
				if err != nil {
					db.DPrintf(db.PYPROXYSRV_ERR, "reader: err starting %v", err)
					return
				}
				db.DPrintf(db.PYPROXYSRV, "reader: Started finished")
			} else if strings.HasPrefix(reqPath, "/Exited") {
				// pps.sc.Exited(0)
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
