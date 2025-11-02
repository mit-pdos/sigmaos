package hotel

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	cachegrpclnt "sigmaos/apps/cache/cachegrp/clnt"
	cachegrpmgr "sigmaos/apps/cache/cachegrp/mgr"
	cossimsrv "sigmaos/apps/cossim/srv"
	"sigmaos/apps/epcache"
	epsrv "sigmaos/apps/epcache/srv"
	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/proc"
	"sigmaos/rpc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

const (
	HOTEL        = "hotel/"
	HOTELDIR     = "name/" + HOTEL
	HOTELGEODIR  = HOTELDIR + "geo/"
	HOTELRATE    = HOTELDIR + "rate"
	HOTELSEARCH  = HOTELDIR + "search"
	HOTELMATCH   = HOTELDIR + "match"
	HOTELREC     = HOTELDIR + "rec"
	HOTELRESERVE = HOTELDIR + "reserve"
	HOTELUSER    = HOTELDIR + "user"
	HOTELPROF    = HOTELDIR + "prof"

	MEMFS          = "memfs"
	HTTP_ADDRS     = "http-addr"
	TRACING        = false
	N_RPC_SESSIONS = 10
)

var HOTELSVC = []string{
	HOTELGEODIR + sp.ANY + "/",
	HOTELRATE,
	HOTELSEARCH,
	HOTELMATCH,
	HOTELREC,
	HOTELRESERVE,
	HOTELUSER,
	HOTELPROF,
	sp.DB + sp.ANY + "/",
}

type HotelJobConfig struct {
	Job             string                      `json:"job"`
	Srvs            []*Srv                      `json:"srvs"`
	NHotel          int                         `json:"n_hotel"`
	Cache           string                      `json:"cache"`
	CacheCfg        *cachegrpmgr.CacheJobConfig `json:"cache_cfg"`
	ImgSizeMB       int                         `json:"img_size_mb"`
	NGeo            int                         `json:"n_geo"`
	NGeoIdx         int                         `json:"n_geo_idx"`
	GeoSearchRadius int                         `json:"geo_search_radius"`
	GeoNResults     int                         `json:"geo_n_results"`
	UseMatch        bool                        `json:"use_match"`
}

func NewHotelJobConfig(job string, srvs []*Srv, nhotel int, cache string, cacheCfg *cachegrpmgr.CacheJobConfig, imgSizeMB int, ngeo int, ngeoidx int, geoSearchRadius int, geoNResults int, useMatch bool) *HotelJobConfig {
	return &HotelJobConfig{
		Job:             job,
		Srvs:            srvs,
		NHotel:          nhotel,
		Cache:           cache,
		CacheCfg:        cacheCfg,
		ImgSizeMB:       imgSizeMB,
		NGeo:            ngeo,
		NGeoIdx:         ngeoidx,
		GeoSearchRadius: geoSearchRadius,
		GeoNResults:     geoNResults,
		UseMatch:        useMatch,
	}
}

func (cfg *HotelJobConfig) String() string {
	return fmt.Sprintf("&{ Job:%v Srvs:%v NHotel:%v Cache:%v CacheCfg:%v ImgSizeMB:%v NGeo:%v NGeoIdx:%v GeoSearchRadius:%v GeoNResults:%v UseMatch:%v }",
		cfg.Job, cfg.Srvs, cfg.NHotel, cfg.Cache, cfg.CacheCfg, cfg.ImgSizeMB, cfg.NGeo, cfg.NGeoIdx, cfg.GeoSearchRadius, cfg.GeoNResults, cfg.UseMatch)
}

var (
	nhotel    int
	imgSizeMB int
)

func init() {
	nh := os.Getenv("NHOTEL")
	if nh == "" {
		// Defaults to 80, same as original DSB.
		nhotel = 80
	} else {
		i, err := strconv.Atoi(nh)
		if err != nil {
			db.DFatalf("Can't parse nhotel: %v", err)
		}
		setNHotel(i)
	}
	isb := os.Getenv("HOTEL_IMG_SZ_MB")
	if isb != "" {
		i, err := strconv.Atoi(isb)
		if err != nil {
			db.DFatalf("Can't parse imgSize: %v", err)
		}
		imgSizeMB = i
	}
}

func setNHotel(nh int) {
	nhotel = nh
}

func JobDir(job string) string {
	return filepath.Join(HOTELDIR, job)
}

func JobHTTPAddrsPath(job string) string {
	return filepath.Join(JobDir(job), HTTP_ADDRS)
}

func MemFsPath(job string) string {
	return filepath.Join(JobDir(job), MEMFS)
}

func NewFsLib(uname string, npc *dialproxyclnt.DialProxyClnt) (*fslib.FsLib, error) {
	pe := proc.GetProcEnv()
	pen := proc.NewAddedProcEnv(pe)
	fsl, err := sigmaclnt.NewFsLib(pen, npc)
	if err != nil {
		db.DPrintf(db.ERROR, "Error newfsl: %v", err)
		return nil, err
	}
	return fsl, nil
}

func GetJobHTTPAddrs(fsl *fslib.FsLib, job string) (sp.Taddrs, error) {
	ep, err := fsl.ReadEndpoint(JobHTTPAddrsPath(job))
	if err != nil {
		return nil, err
	}
	return ep.Addrs(), err
}

func InitHotelFs(fsl *fslib.FsLib, jobname string) error {
	fsl.MkDir(HOTELDIR, 0777)
	fsl.MkDir(HOTELGEODIR, 0777)
	if err := fsl.MkDir(JobDir(jobname), 0777); err != nil {
		db.DPrintf(db.ERROR, "Mkdir %v err %v\n", JobDir(jobname), err)
		return err
	}
	return nil
}

type Srv struct {
	Name string
	Args []string
	Mcpu proc.Tmcpu
}

var geo *Srv = &Srv{"hotel-geod", nil, 2000}

// XXX searchd only needs 2, but in order to make spawns work out we need to have it run with 3.
func NewHotelSvc() []*Srv {
	return []*Srv{
		&Srv{"hotel-userd", nil, 0},
		&Srv{"hotel-rated", nil, 2000},
		geo,
		&Srv{"hotel-profd", nil, 2000},
		&Srv{"hotel-searchd", nil, 3000},
		&Srv{"hotel-reserved", nil, 3000},
		&Srv{"hotel-recd", nil, 0},
		&Srv{"hotel-wwwd", nil, 3000},
	}
}

// XXX searchd only needs 2, but in order to make spawns work out we need to have it run with 3.
func NewHotelSvcWithMatch() []*Srv {
	return []*Srv{
		&Srv{"hotel-userd", nil, 0},
		&Srv{"hotel-rated", nil, 2000},
		geo,
		&Srv{"hotel-profd", nil, 2000},
		&Srv{"hotel-searchd", nil, 3000},
		&Srv{"hotel-matchd", nil, 3000},
		&Srv{"hotel-reserved", nil, 3000},
		&Srv{"hotel-recd", nil, 0},
		&Srv{"hotel-wwwd", nil, 3000},
	}
}

type HotelJob struct {
	*sigmaclnt.SigmaClnt
	EPCacheJob      *epsrv.EPCacheJob
	cacheClnt       *cachegrpclnt.CachedSvcClnt
	CacheMgr        *cachegrpmgr.CacheMgr
	CacheAutoscaler *cachegrpclnt.Autoscaler
	CosSimJob       *cossimsrv.CosSimJob
	pids            []sp.Tpid
	cache           string
	job             string
	epcsrvEP        *sp.Tendpoint
}

func NewHotelJob(sc *sigmaclnt.SigmaClnt, cfg *HotelJobConfig, csjConf *cossimsrv.CosSimJobConfig) (*HotelJob, error) {
	// Set number of hotels before doing anything.
	setNHotel(cfg.NHotel)
	// Set the number of indexes to be used in each geo server
	geo.Args = []string{strconv.Itoa(cfg.NGeoIdx), strconv.Itoa(cfg.GeoSearchRadius), strconv.Itoa(cfg.GeoNResults)}

	var cc *cachegrpclnt.CachedSvcClnt
	var cm *cachegrpmgr.CacheMgr
	var ca *cachegrpclnt.Autoscaler
	var err error

	// Init fs.
	if err := InitHotelFs(sc.FsLib, cfg.Job); err != nil {
		return nil, err
	}

	// Create epcache job
	epcj, err := epsrv.NewEPCacheJob(sc)
	if err != nil {
		return nil, err
	}

	// Read the endpoint of the endpoint cache server
	epcsrvEPB, err := sc.GetFile(epcache.EPCACHE)
	if err != nil {
		return nil, err
	}

	epcsrvEP, err := sp.NewEndpointFromBytes(epcsrvEPB)
	if err != nil {
		return nil, err
	}

	// Create a cache clnt.
	if cfg.CacheCfg.NSrv > 0 {
		switch cfg.Cache {
		case "cached":
			db.DPrintf(db.ALWAYS, "Hotel running with cached")
			cm, err = cachegrpmgr.NewCacheMgrEPCache(sc, epcj, cfg.Job, cfg.CacheCfg)
			if err != nil {
				db.DPrintf(db.ERROR, "Error NewCacheMgr %v", err)
				return nil, err
			}
			cc = cachegrpclnt.NewCachedSvcClntEPCache(sc.FsLib, epcj.Clnt, cfg.Job)
			ca = cachegrpclnt.NewAutoscaler(cm, cc)
		// XXX Remove
		case "memcached":
			db.DPrintf(db.ALWAYS, "Hotel running with memcached")
		default:
			db.DPrintf(db.ERROR, "Unrecognized hotel cache type: %v", cfg.Cache)
		}
	}

	// Initialize CosSimJob after cache client is created
	var cosSimJob *cossimsrv.CosSimJob
	if cfg.UseMatch {
		db.DPrintf(db.TEST, "Start cossimsrv")
		cosSimJob, err = cossimsrv.NewCosSimJob(csjConf, sc, epcj, cm, cc)
		if err != nil {
			db.DPrintf(db.ERROR, "Error NewCosSimJob %v", err)
			return nil, err
		}
		if _, _, err := cosSimJob.AddSrv(); err != nil {
			db.DPrintf(db.ERROR, "Error CosSimJob.AddSrv %v", err)
			return nil, err
		}
		db.DPrintf(db.TEST, "Done start cossimsrv")
	}

	pids := make([]sp.Tpid, 0, len(cfg.Srvs))

	for _, srv := range cfg.Srvs {
		db.DPrintf(db.TEST, "Hotel spawn %v", srv.Name)
		p := proc.NewProc(srv.Name, append([]string{cfg.Job, cfg.Cache}, srv.Args...))
		p.SetCachedEndpoint(epcache.EPCACHE, epcsrvEP)
		p.AppendEnv("NHOTEL", strconv.Itoa(cfg.NHotel))
		p.AppendEnv("HOTEL_IMG_SZ_MB", strconv.Itoa(cfg.ImgSizeMB))
		p.SetMcpu(srv.Mcpu)
		if err := sc.Spawn(p); err != nil {
			db.DPrintf(db.ERROR, "Error spawn proc %v: %v", p, err)
			return nil, err
		}
		if err = sc.WaitStart(p.GetPid()); err != nil {
			db.DPrintf(db.ERROR, "Error start proc %v: %v", p, err)
			return nil, err
		}
		pids = append(pids, p.GetPid())
		db.DPrintf(db.TEST, "Hotel started %v", srv.Name)
	}

	hj := &HotelJob{
		SigmaClnt:       sc,
		EPCacheJob:      epcj,
		cacheClnt:       cc,
		CacheMgr:        cm,
		CacheAutoscaler: ca,
		CosSimJob:       cosSimJob,
		pids:            pids,
		cache:           cfg.Cache,
		job:             cfg.Job,
		epcsrvEP:        epcsrvEP,
	}

	if cfg.NGeo > 1 {
		for i := 0; i < cfg.NGeo-1; i++ {
			if err := hj.AddGeoSrv(); err != nil {
				return nil, err
			}
		}
	}

	return hj, nil
}

func (hj *HotelJob) AddCosSimSrv() error {
	return hj.AddCosSimSrvWithSigmaPath(sp.NOT_SET)
}

func (hj *HotelJob) RemoveCosSimSrv() error {
	return hj.CosSimJob.RemoveSrv()
}

func (hj *HotelJob) AddCosSimSrvWithSigmaPath(sigmaPath string) error {
	_, _, err := hj.CosSimJob.AddSrvWithSigmaPath(sigmaPath)
	return err
}

func (hj *HotelJob) AddGeoSrv() error {
	p := proc.NewProc(geo.Name, append([]string{hj.job, hj.cache}, geo.Args...))
	p.SetCachedEndpoint(epcache.EPCACHE, hj.epcsrvEP)
	p.AppendEnv("NHOTEL", strconv.Itoa(nhotel))
	p.AppendEnv("HOTEL_IMG_SZ_MB", strconv.Itoa(imgSizeMB))
	p.SetMcpu(geo.Mcpu)
	db.DPrintf(db.TEST, "Hotel spawn additional %v", geo.Name)
	db.DPrintf(db.TEST, "Hotel spawn additional %v %v", geo.Name, p)
	if err := hj.Spawn(p); err != nil {
		db.DPrintf(db.ERROR, "Error spawn proc %v: %v", p, err)
		return err
	}
	if err := hj.WaitStart(p.GetPid()); err != nil {
		db.DPrintf(db.ERROR, "Error start proc %v: %v", p, err)
		return err
	}
	hj.pids = append(hj.pids, p.GetPid())
	db.DPrintf(db.TEST, "Hotel started %v", geo.Name)
	return nil
}

func (hj *HotelJob) Stop() error {
	if hj.CacheAutoscaler != nil {
		hj.CacheAutoscaler.Stop()
	}
	for _, pid := range hj.pids {
		if err := hj.Evict(pid); err != nil {
			return err
		}
		if _, err := hj.WaitExit(pid); err != nil {
			return err
		}
	}
	if hj.CosSimJob != nil {
		hj.CosSimJob.Stop()
	}
	if hj.CacheMgr != nil {
		hj.CacheMgr.Stop()
	}
	hj.EPCacheJob.Stop()
	return nil
}

func (hj *HotelJob) StatsSrv() ([]*rpc.RPCStatsSnapshot, error) {
	if hj.cacheClnt != nil {
		return hj.cacheClnt.StatsSrvs()
	}
	db.DPrintf(db.ALWAYS, "No cacheclnt")
	return nil, nil
}
