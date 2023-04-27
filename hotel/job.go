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
	HOTEL          = "hotel"
	HOTELDIR       = "name/hotel/"
	MEMFS          = "memfs"
	HTTP_ADDRS     = "http-addr"
	TRACING        = false
	N_RPC_SESSIONS = 10
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

func MakeFsLibs(uname string) []*fslib.FsLib {
	fsls := make([]*fslib.FsLib, 0, N_RPC_SESSIONS)
	for i := 0; i < N_RPC_SESSIONS; i++ {
		fsl, err := fslib.MakeFsLib(uname + "-" + strconv.Itoa(i))
		if err != nil {
			db.DFatalf("Error mkfsl: %v", err)
		}
		fsls = append(fsls, fsl)
	}
	return fsls
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

// XXX searchd only needs 2, but in order to make spawns work out we need to have it run with 3.
func MkHotelSvc(public bool) []Srv {
	return []Srv{
		Srv{"hotel-userd", public, 0}, Srv{"hotel-rated", public, 2},
		Srv{"hotel-geod", public, 2}, Srv{"hotel-profd", public, 2},
		Srv{"hotel-searchd", public, 3}, Srv{"hotel-reserved", public, 3},
		Srv{"hotel-recd", public, 0}, Srv{"hotel-wwwd", public, 3},
	}
}

//var ncores = []int{0, 1,
//	1, 1, 3,
//	3, 0, 2}

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

func MakeHotelJob(sc *sigmaclnt.SigmaClnt, job string, srvs []Srv, cache string, cacheNcore proc.Tcore, nsrv int, gc bool) (*HotelJob, error) {
	var cc *cacheclnt.CacheClnt
	var cm *cacheclnt.CacheMgr
	var err error
	var kvf *kv.KVFleet
	// Init fs.
	InitHotelFs(sc.FsLib, job)

	// Create a cache clnt.
	if nsrv > 0 {
		switch cache {
		case "cached":
			db.DPrintf(db.ALWAYS, "Hotel running with cached")
			cm, err = cacheclnt.MkCacheMgr(sc, job, nsrv, cacheNcore, gc, test.Overlays)
			if err != nil {
				db.DFatalf("Error MkCacheMgr %v", err)
				return nil, err
			}
			cc, err = cacheclnt.MkCacheClnt([]*fslib.FsLib{sc.FsLib}, job)
			if err != nil {
				db.DFatalf("Error cacheclnt %v", err)
				return nil, err
			}
		case "kvd":
			db.DPrintf(db.ALWAYS, "Hotel running with kvd")
			kvf, err = kv.MakeKvdFleet(sc, job, nsrv, 0, cacheNcore, "0", "manual")
			if err != nil {
				return nil, err
			}
			err = kvf.Start()
			if err != nil {
				return nil, err
			}
		// XXX Remove
		case "memcached":
			db.DPrintf(db.ALWAYS, "Hotel running with memcached")
		default:
			db.DFatalf("Unrecognized hotel cache type: %v", cache)
		}
	}

	pids := make([]proc.Tpid, 0, len(srvs))

	for _, srv := range srvs {
		p := proc.MakeProc(srv.Name, []string{job, strconv.FormatBool(srv.Public), cache})
		//		p.AppendEnv("GOGC", "off")
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

func (hj *HotelJob) StatsSrv() ([]*protdev.SigmaRPCStats, error) {
	if hj.cacheClnt != nil {
		return hj.cacheClnt.StatsSrv()
	}
	db.DPrintf(db.ALWAYS, "No cacheclnt")
	return nil, nil
}
