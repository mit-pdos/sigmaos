package config

import (
	"sigmaos/atomic"
	db "sigmaos/debug"
	"sigmaos/fslib"
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
func (clnt *ConfigClnt) WaitConfigChange(path string) error {
	done := make(chan bool)
	err := clnt.SetRemoveWatch(path, func(path string, err error) {
		if err != nil {
			db.DPrintf(db.ALWAYS, "Error in SetRemoveWatch: %v", err)
		}
		done <- true
	})
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error SetRemoveWatch in ConfigClnt: %v", err)
		return err
	}
	<-done
	return nil
}

// Read the config stored at path into cfg. Will block until the config file
// becomes available.
func (clnt *ConfigClnt) ReadConfig(path string, cfg interface{}) error {
	err := clnt.GetFileJsonWatch(path, cfg)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error GetFileJsonWatch in ConfigClnt: %v", err)
		return err
	}
	return nil
}

// Write cfg into path
func (clnt *ConfigClnt) WriteConfig(path string, cfg interface{}) error {
	// Make the realm config file.
	if err := atomic.PutFileJsonAtomic(clnt.FsLib, path, 0777, cfg); err != nil {
		db.DPrintf(db.ALWAYS, "Error MakeFileJsonAtomic in ConfigClnt.WriteConfig: %v", err)
		return err
	}
	return nil
}
