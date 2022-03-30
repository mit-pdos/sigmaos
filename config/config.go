package config

import (
	"runtime/debug"

	"ulambda/atomic"
	db "ulambda/debug"
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
		if err != nil && np.IsErrUnreachable(err) {
			db.DFatalf("Error Watch in ConfigClnt.WatchConfig: %v", err)
		}
		done <- true
	})
	if err != nil && np.IsErrUnreachable(err) {
		debug.PrintStack()
		db.DFatalf("Error SetRemoveWatch in ConfigClnt.WatchConfig: %v", err)
	}
	return done
}

// Read the config stored at path into cfg. Will block until the config file
// becomes available.
func (clnt *ConfigClnt) ReadConfig(path string, cfg interface{}) {
	for {
		err := clnt.GetFileJson(path, cfg)
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
	if err := atomic.PutFileJsonAtomic(clnt.FsLib, path, 0777, cfg); err != nil {
		debug.PrintStack()
		db.DFatalf("Error MakeFileJsonAtomic in ConfigClnt.WriteConfig: %v", err)
	}
}
