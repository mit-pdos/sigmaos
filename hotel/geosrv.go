package hotel

import (
	"encoding/json"
	"log"
	"math/rand"
	"strconv"
	"sync"

	//	"go.opentelemetry.io/otel/trace"

	"github.com/harlow/go-micro-services/data"
	"github.com/mit-pdos/go-geoindex"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/hotel/proto"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/sigmasrv"
	"sigmaos/tracing"
)

const (
	N_INDEX   = 1000
	RAND_SEED = 12345
)

const (
	maxSearchRadius  = 10
	maxSearchResults = 5
)

type safeIndex struct {
	mu     sync.Mutex
	geoidx *geoindex.ClusteringIndex
}

func newSafeIndex(path string) *safeIndex {
	return &safeIndex{
		geoidx: newGeoIndex(path),
	}
}

func (si *safeIndex) KNN(center *geoindex.GeoPoint) []geoindex.Point {
	si.mu.Lock()
	defer si.mu.Unlock()

	return si.geoidx.KNearest(
		center,
		maxSearchResults,
		geoindex.Km(maxSearchRadius), func(p geoindex.Point) bool {
			return true
		},
	)
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
	tracer  *tracing.Tracer
	indexes []*safeIndex
}

// Run starts the server
func RunGeoSrv(job string) error {
	rand.Seed(RAND_SEED)
	geo := &Geo{}
	geo.indexes = make([]*safeIndex, 0, N_INDEX)
	for i := 0; i < N_INDEX; i++ {
		geo.indexes = append(geo.indexes, newSafeIndex("data/geo.json"))
	}
	ssrv, err := sigmasrv.NewSigmaSrv(HOTELGEO, geo, proc.GetProcEnv())
	if err != nil {
		return err
	}

	p, err := perf.NewPerf(ssrv.MemFs.SigmaClnt().ProcEnv(), perf.HOTEL_GEO)
	if err != nil {
		db.DFatalf("NewPerf err %v\n", err)
	}
	defer p.Done()
	//	geo.tracer = tracing.Init("geo", proc.GetSigmaJaegerIP())
	//	defer geo.tracer.Flush()

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

	r := rand.Int63() % N_INDEX

	si := s.indexes[r]

	return si.KNN(center)
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
