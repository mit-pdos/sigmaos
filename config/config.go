package config

import (
	"log"
	"runtime/debug"

	"ulambda/atomic"
	"ulambda/fslib"
	np "ulambda/ninep"
)

type ConfigClnt struct {
	*fslib.FsLib
}

func MakeConfigClnt(fsl *fslib.FsLib) *ConfigClnt {
	clnt := &ConfigClnt{}
	clnt.FsLib = fsl
	return clnt
}

// Watch a config file
func (clnt *ConfigClnt) WatchConfig(path string) chan bool {
	done := make(chan bool)
	err := clnt.SetRemoveWatch(path, func(path string, err error) {
		if err != nil && np.IsErrEOF(err) {
			log.Fatalf("Error Watch in ConfigClnt.WatchConfig: %v", err)
		}
		done <- true
	})
	if err != nil && np.IsErrEOF(err) {
		debug.PrintStack()
		log.Fatalf("Error SetRemoveWatch in ConfigClnt.WatchConfig: %v", err)
	}
	return done
}

// Read the config stored at path into cfg. Will block until the config file
// becomes available.
func (clnt *ConfigClnt) ReadConfig(path string, cfg interface{}) {
	for {
		err := clnt.ReadFileJson(path, cfg)
		if err == nil {
			break
		}
		done := clnt.WatchConfig(path)
		<-done
	}
}

// Write cfg into path
func (clnt *ConfigClnt) WriteConfig(path string, cfg interface{}) {
	// Make the realm config file.
	if err := atomic.MakeFileJsonAtomic(clnt.FsLib, path, 0777, cfg); err != nil {
		debug.PrintStack()
		log.Fatalf("Error MakeFileJsonAtomic in ConfigClnt.WriteConfig: %v", err)
	}
}
