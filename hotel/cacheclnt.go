package hotel

import (
	"encoding/json"

	"sigmaos/fslib"
	"sigmaos/protdevclnt"
)

type CacheClnt struct {
	cachec *protdevclnt.ProtDevClnt
}

func MkCacheClnt(fsl *fslib.FsLib, fn string) (*CacheClnt, error) {
	c := &CacheClnt{}
	pdc, err := protdevclnt.MkProtDevClnt(fsl, fn)
	if err != nil {
		return nil, err
	}
	c.cachec = pdc
	return c, nil
}

func (c *CacheClnt) Set(key string, val any) error {
	req := &CacheRequest{}
	req.Key = key
	b, err := json.Marshal(val)
	if err != nil {
		return nil
	}
	req.Value = b
	var res CacheResult
	if err := c.cachec.RPC("Cache.Set", req, &res); err != nil {
		return err
	}
	return nil
}

func (c *CacheClnt) Get(key string, val any) error {
	req := &CacheRequest{}
	req.Key = key
	var res CacheResult
	if err := c.cachec.RPC("Cache.Get", req, &res); err != nil {
		return err
	}
	if err := json.Unmarshal(res.Value, val); err != nil {
		return err
	}
	return nil
}
