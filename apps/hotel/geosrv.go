package hotel

import (
	"encoding/json"
	"log"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	//	"go.opentelemetry.io/otel/trace"

	"github.com/harlow/go-micro-services/data"
	"github.com/mit-pdos/go-geoindex"

	"sigmaos/apps/hotel/proto"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmasrv"
	"sigmaos/tracing"
	"sigmaos/util/perf"
)

type GeoIndexes struct {
	mu      sync.Mutex
	indexes chan *geoindex.ClusteringIndex
}

func NewGeoIndexes(n int, path string) *GeoIndexes {
	idxs := &GeoIndexes{
		indexes: make(chan *geoindex.ClusteringIndex, n),
	}
	for i := 0; i < n; i++ {
		idxs.indexes <- newGeoIndex(path)
	}
	return idxs
}

func (gi GeoIndexes) KNN(center *geoindex.GeoPoint, maxSearchRadius float64, maxSearchResults int) []geoindex.Point {
	idx := <-gi.indexes
	start := time.Now()
	points := idx.KNearest(
		center,
		maxSearchResults,
		geoindex.Km(maxSearchRadius), func(p geoindex.Point) bool {
			return true
		},
	)
	db.DPrintf(db.ALWAYS, "Time KNN (%v): %v", center, time.Since(start))
	gi.indexes <- idx
	return points
}

// Point represents a hotels's geo location on map
type point struct {
	Pid  string  `json:"hotelId"`
	Plat float64 `json:"lat"`
	Plon float64 `json:"lon"`
}

// Implement Point interface
func (p *point) Lat() float64 { return p.Plat }
func (p *point) Lon() float64 { return p.Plon }
func (p *point) Id() string   { return p.Pid }

// Server implements the geo service
type Geo struct {
	tracer           *tracing.Tracer
	idxs             *GeoIndexes
	maxSearchRadius  float64
	maxSearchResults int
}

// Run starts the server
func RunGeoSrv(job string, ckptpn string, nidxStr string, maxSearchRadiusStr string, maxSearchResultsStr string) error {
	db.DPrintf(db.CKPT, "start %v %v\n", job, ckptpn)
	nidx, err := strconv.Atoi(nidxStr)
	if err != nil {
		db.DFatalf("Invalid nidx: %v", err)
	}
	maxSearchRadius, err := strconv.Atoi(maxSearchRadiusStr)
	if err != nil {
		db.DFatalf("Invalid maxSearchRadiusStr: %v", err)
	}
	maxSearchResults, err := strconv.Atoi(maxSearchResultsStr)
	if err != nil {
		db.DFatalf("Invalid maxSearchResults: %v", err)
	}
	geo := &Geo{
		maxSearchRadius:  float64(maxSearchRadius),
		maxSearchResults: maxSearchResults,
	}
	start := time.Now()
	geo.idxs = NewGeoIndexes(nidx, "data/geo.json")
	db.DPrintf(db.CKPT, "init done %v\n", job)
	db.DPrintf(db.ALWAYS, "Geo srv done building %v indexes, radius %v nresults %v,  after: %v", nidx, geo.maxSearchRadius, geo.maxSearchResults, time.Since(start))

	if ckptpn != "" {
		// create a sigmaclnt for checkpointing
		sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
		if err != nil {
			db.DFatalf("NewSigmaClnt error %v\n", err)
		}
		err = sc.Started()
		if err != nil {
			db.DFatalf("Started error %v\n", err)
		}
		sc, err = sc.CheckpointMe(ckptpn)
		if err != nil {
			db.DFatalf("Checkpoint me didn't return error: %v", err)
		}
		db.DPrintf(db.ALWAYS, "Mark started")
		err = sc.Started()
		if err != nil {
			db.DFatalf("Started error %v\n", err)
		}
		sc.Close()
	}
	db.DPrintf(db.ALWAYS, "Making env")
	pe := proc.GetProcEnv()
	ssrv, err := sigmasrv.NewSigmaSrv(filepath.Join(HOTELGEODIR, pe.GetPID().String()), geo, pe)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error starting sigmasrv")
		return err
	}
	db.DPrintf(db.ALWAYS, "Making perf")
	p, err := perf.NewPerf(ssrv.MemFs.SigmaClnt().ProcEnv(), perf.HOTEL_GEO)
	if err != nil {
		db.DFatalf("NewPerf err %v\n", err)
	}
	defer p.Done()
	//	geo.tracer = tracing.Init("geo", proc.GetSigmaJaegerIP())
	//	defer geo.tracer.Flush()

	db.DPrintf(db.ALWAYS, "Geo srv ready to serve time since spawn: %v", time.Since(ssrv.ProcEnv().GetSpawnTime()))
	return ssrv.RunServer()
}

// Nearby returns all hotels within a given distance.
func (s *Geo) Nearby(ctx fs.CtxI, req proto.GeoRequest, rep *proto.GeoResult) error {
	//	var span trace.Span
	//	if TRACING {
	//		_, span = s.tracer.StartRPCSpan(&req, "Nearby")
	//		defer span.End()
	//	}

	db.DPrintf(db.HOTEL_GEO, "Nearby %v\n", req)
	points := s.getNearbyPoints(float64(req.Lat), float64(req.Lon))
	for _, p := range points {
		rep.HotelIds = append(rep.HotelIds, p.Id())
	}
	return nil
}

func (s *Geo) getNearbyPoints(lat, lon float64) []geoindex.Point {
	center := &geoindex.GeoPoint{
		Pid:  "",
		Plat: lat,
		Plon: lon,
	}
	return s.idxs.KNN(center, s.maxSearchRadius, s.maxSearchResults)
}

// newGeoIndex returns a geo index with points loaded
func newGeoIndex(path string) *geoindex.ClusteringIndex {
	var (
		file   = data.MustAsset(path)
		points []*point
	)

	// load geo points from json file
	if err := json.Unmarshal(file, &points); err != nil {
		log.Fatalf("Failed to load hotels: %v", err)
	}
	// add points to index
	index := geoindex.NewClusteringIndex()
	for _, point := range points {
		index.Add(point)
	}
	for i := 7; i < nhotel; i++ {
		p := &geoindex.GeoPoint{
			Pid:  strconv.Itoa(i),
			Plat: 37.7835 + float64(i)/500.0*3,
			Plon: -122.41 + float64(i)/500.0*4,
		}
		index.Add(p)
	}
	return index
}
