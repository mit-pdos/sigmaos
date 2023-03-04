package hotel

import (
	"path"
	"strconv"

	"sigmaos/cacheclnt"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/kv"
	"sigmaos/proc"
	"sigmaos/protdev"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	HOTEL      = "hotel"
	HOTELDIR   = "name/hotel/"
	MEMFS      = "memfs"
	HTTP_ADDRS = "http-addr"
)

func JobDir(job string) string {
	return path.Join(HOTELDIR, job)
}

func JobHTTPAddrsPath(job string) string {
	return path.Join(JobDir(job), HTTP_ADDRS)
}

func MemFsPath(job string) string {
	return path.Join(JobDir(job), MEMFS)
}

func GetJobHTTPAddrs(fsl *fslib.FsLib, job string) (sp.Taddrs, error) {
	mnt, err := fsl.ReadMount(JobHTTPAddrsPath(job))
	if err != nil {
		return nil, err
	}
	return mnt.Addr, err
}

func InitHotelFs(fsl *fslib.FsLib, jobname string) {
	fsl.MkDir(HOTELDIR, 0777)
	if err := fsl.MkDir(JobDir(jobname), 0777); err != nil {
		db.DFatalf("Mkdir %v err %v\n", JobDir(jobname), err)
	}
}

type Srv struct {
	Name   string
	Public bool
	Ncore  proc.Tcore
}

func MkHotelSvc(public bool) []Srv {
	return []Srv{
		Srv{"hotel-userd", public, 0}, Srv{"hotel-rated", public, 2},
		Srv{"hotel-geod", public, 2}, Srv{"hotel-profd", public, 2},
		Srv{"hotel-searchd", public, 3}, Srv{"hotel-reserved", public, 3},
		Srv{"hotel-recd", public, 0}, Srv{"hotel-wwwd", public, 2},
	}
}

//var ncores = []int{0, 1,
//	1, 1, 3,
//	3, 0, 2}

var cacheNcore = 2

//var ncores = []int{0, 2,
//	2, 2, 3,
//	3, 0, 2}

type HotelJob struct {
	*sigmaclnt.SigmaClnt
	cacheClnt *cacheclnt.CacheClnt
	cacheMgr  *cacheclnt.CacheMgr
	pids      []proc.Tpid
	cache     string
	kvf       *kv.KVFleet
}

func MakeHotelJob(sc *sigmaclnt.SigmaClnt, job string, srvs []Srv, cache string, nsrv int) (*HotelJob, error) {
	var cc *cacheclnt.CacheClnt
	var cm *cacheclnt.CacheMgr
	var err error
	var kvf *kv.KVFleet
	// Init fs.
	InitHotelFs(sc.FsLib, job)

	// Create a cache clnt.
	if nsrv > 0 {
		if cache == "cached" {
			db.DPrintf(db.ALWAYS, "Hotel running with cached")
			cm, err = cacheclnt.MkCacheMgr(sc, job, nsrv, proc.Tcore(cacheNcore), test.Overlays)
			if err != nil {
				db.DFatalf("Error MkCacheMgr %v", err)
				return nil, err
			}
			cc, err = cacheclnt.MkCacheClnt(sc.FsLib, job)
			if err != nil {
				db.DFatalf("Error cacheclnt %v", err)
				return nil, err
			}
		} else {
			db.DPrintf(db.ALWAYS, "Hotel running with kvd")
			kvf, err = kv.MakeKvdFleet(sc, job, nsrv, 0, proc.Tcore(cacheNcore), "0", "manual")
			if err != nil {
				return nil, err
			}
			err = kvf.Start()
			if err != nil {
				return nil, err
			}
		}
	}

	pids := make([]proc.Tpid, 0, len(srvs))

	for _, srv := range srvs {
		p := proc.MakeProc(srv.Name, []string{job, strconv.FormatBool(srv.Public), cache})
		p.SetNcore(srv.Ncore)
		if _, errs := sc.SpawnBurst([]*proc.Proc{p}, 2); len(errs) > 0 {
			db.DFatalf("Error burst-spawnn proc %v: %v", p, errs)
			return nil, err
		}
		if err = sc.WaitStart(p.GetPid()); err != nil {
			db.DFatalf("Error spawn proc %v: %v", p, err)
			return nil, err
		}
		pids = append(pids, p.GetPid())
	}

	return &HotelJob{sc, cc, cm, pids, cache, kvf}, nil
}

func (hj *HotelJob) Stop() error {
	for _, pid := range hj.pids {
		if err := hj.Evict(pid); err != nil {
			return err
		}
		if _, err := hj.WaitExit(pid); err != nil {
			return err
		}
	}
	if hj.cacheMgr != nil {
		hj.cacheMgr.Stop()
	}
	if hj.kvf != nil {
		hj.kvf.Stop()
	}
	return nil
}

func (hj *HotelJob) StatsSrv() ([]*protdev.Stats, error) {
	if hj.cacheClnt != nil {
		return hj.cacheClnt.StatsSrv()
	}
	db.DPrintf(db.ALWAYS, "No cacheclnt")
	return nil, nil
}
