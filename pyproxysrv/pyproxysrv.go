package pyproxysrv

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"

	"github.com/rogpeppe/go-internal/dirhash"
)

const (
	CHECKSUM         = "sigmaos-checksum"
	CHECKSUMOVERRIDE = "sigmaos-checksum-override"

	PYTHON   = "/tmp/python"
	LIB      = "/tmp/python/Lib"
	SUPERLIB = "/tmp/python/superlib"
)

// PyProxySrv maintains the state of the pyproxysrv.
type PyProxySrv struct {
	pe         *proc.ProcEnv
	sc         *sigmaclnt.SigmaClnt
	bn         string // Name of AWS bucket
	relImports bool
}

// Creates and returns a new PyProxySrv object to be used by Python programs
// for fetching Python libraries and interacting with the SigmaOS API.
func NewPyProxySrv(pe *proc.ProcEnv, bn string) (*PyProxySrv, error) {
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
		pe:         pe,
		bn:         bn,
		relImports: false,
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

// Computes the checksum for the given file
func (pp *PyProxySrv) libFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		db.DPrintf(db.PYPROXYSRV_ERR, "Err opening %v: %v", filePath, err)
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	_, err = io.Copy(hasher, file)
	if err != nil {
		db.DPrintf(db.PYPROXYSRV_ERR, "Err hashing: %v", err)
		return "", err
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// Computes the checksum for the given directory
func (pps *PyProxySrv) libDirChecksum(dirPath string) (string, error) {
	hash, err := dirhash.HashDir(dirPath, "", dirhash.Hash1)
	if err != nil {
		db.DPrintf(db.PYPROXYSRV_ERR, "Err hashing directory %v: %v", dirPath, err)
		return "", err
	}

	checksumPath := filepath.Join(dirPath, CHECKSUM)
	checksum := []byte(hash)
	err = ioutil.WriteFile(checksumPath, checksum, 0777)
	if err != nil {
		return "", err
	}
	return hash, nil
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

// Wrapper to fetch the specified library from the user's AWS bucket.
func (pps *PyProxySrv) fetchLib(libName string) {
	db.DPrintf(db.PYPROXYSRV, "Fetching %v", libName)
	pn := filepath.Join(sp.NAMED, "s3", "~any", pps.bn, libName)
	libDest := filepath.Join(LIB, libName)

	err := pps.copyLib(pn, libDest)
	if err != nil {
		db.DPrintf(db.PYPROXYSRV_ERR, "copyLib: %v", err)
		return
	}

	db.DPrintf(db.PYPROXYSRV, "Finished fetching %v", libName)

	// Calculate hash
	fileInfo, err := os.Stat(libDest)
	if err != nil {
		db.DPrintf(db.PYPROXYSRV_ERR, "Err checking for %v: %v", libDest, err)
		return
	}
	isDir := fileInfo.IsDir()
	if isDir {
		hash, err := pps.libDirChecksum(libDest)
		if err != nil {
			db.DPrintf(db.PYPROXYSRV_ERR, "Err hashing %v: %v", libDest, err)
			return
		}

		err = ioutil.WriteFile(filepath.Join(libDest, CHECKSUM), []byte(hash), 0777)
		if err != nil {
			db.DPrintf(db.PYPROXYSRV_ERR, "Err writing hash %v: %v", libDest, err)
			return
		}
	} else {
		hash, err := pps.libFileChecksum(libDest)
		if err != nil {
			db.DPrintf(db.PYPROXYSRV_ERR, "Err hashing %v: %v", libDest, err)
			return
		}

		checksumDest := strings.TrimSuffix(libDest, ".py") + "-" + CHECKSUM
		err = ioutil.WriteFile(checksumDest, []byte(hash), 0777)
		if err != nil {
			db.DPrintf(db.PYPROXYSRV_ERR, "Err writing hash %v: %v", libDest, err)
			return
		}
	}

	return
}

// Main proxy body to handle all Python program requests.
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
			db.DPrintf(db.PYPROXYSRV, "reader: received initialization request\n")

			// Set up superlib dummy directory
			err := os.MkdirAll(SUPERLIB, 0777)
			if err != nil {
				db.DPrintf(db.PYPROXYSRV_ERR, "reader: err creating superlib %v\n", err)
			}

			// Record contents at the Lib directory
			libFiles, err := os.ReadDir(LIB)
			if err != nil {
				db.DPrintf(db.PYPROXYSRV_ERR, "reader: err reading Python Lib %v\n", err)
				return
			}

			for _, file := range libFiles {
				// Ensure file name is not a checksum/override
				if strings.HasSuffix(file.Name(), CHECKSUM) || strings.HasSuffix(file.Name(), CHECKSUMOVERRIDE) {
					continue
				}

				db.DPrintf(db.PYPROXYSRV, "reader: Lib: %v\n", file.Name())

				// Check for checksum's presence
				if file.IsDir() {
					_, checksumErr := os.Stat(filepath.Join(LIB, file.Name(), CHECKSUM))
					_, overrideErr := os.Stat(filepath.Join(LIB, file.Name(), CHECKSUMOVERRIDE))
					if (checksumErr != nil) && (overrideErr != nil) {
						continue
					}
				} else {
					libName := strings.TrimSuffix(file.Name(), ".py")
					_, checksumErr := os.Stat(filepath.Join(LIB, libName+"-"+CHECKSUM))
					_, overrideErr := os.Stat(filepath.Join(LIB, libName+"-"+CHECKSUMOVERRIDE))
					if (checksumErr != nil) && (overrideErr != nil) {
						continue
					}
				}

				err = os.WriteFile(filepath.Join(SUPERLIB, file.Name()), []byte(""), 0777)
				if err != nil {
					err = os.MkdirAll(filepath.Join(SUPERLIB, file.Name()), 0777)
					if err != nil {
						db.DPrintf(db.PYPROXYSRV_ERR, "reader: err reading local lib %v]n", err)
						return
					}
				}
				libContents[file.Name()] = true
			}

			// Add all libraries in the S3 bucket to superlib
			pn := filepath.Join(sp.NAMED, "s3", "~any", pps.bn)
			_, err = pps.sc.ProcessDir(pn, func(st *sp.Tstat) (bool, error) {
				err := os.MkdirAll(filepath.Join(SUPERLIB, st.Name), 0777)
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
			if strings.HasPrefix(reqPath, "/Started") {
				db.DPrintf(db.PYPROXYSRV, "reader: Started called")
				err := pps.sc.ProcAPI.Started()
				if err != nil {
					db.DPrintf(db.PYPROXYSRV_ERR, "reader: err starting %v", err)
					return
				}
			} else if strings.HasPrefix(reqPath, "/Exited") {
				pps.sc.ProcAPI.Exited(proc.NewStatus(proc.StatusOK))
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
