package hotel

import (
	"encoding/json"
	"log"

	"github.com/hailocab/go-geoindex"
	"github.com/harlow/go-micro-services/data"
	// "github.com/harlow/go-micro-services/internal/proto/geo"

	"sigmaos/fs"
	np "sigmaos/ninep"
	"sigmaos/protdevsrv"
)

const (
	maxSearchRadius  = 10
	maxSearchResults = 5
)

type GeoRequest struct {
	Lat float64
	Lon float64
}

type GeoResult struct {
	HotelIds []string
}

// point represents a hotels's geo location on map
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
	geoidx *geoindex.ClusteringIndex
}

// Run starts the server
func RunGeoSrv() error {
	geo := &Geo{}
	geo.geoidx = newGeoIndex("data/geo.json")
	protdevsrv.Run(np.HOTELGEO, geo.mkStream)
	return nil
}

type Stream struct {
	rep []byte
	geo *Geo
}

func (geo *Geo) mkStream() (fs.File, *np.Err) {
	st := &Stream{}
	st.geo = geo
	return st, nil
}

// XXX wait on close before processing data?
func (st *Stream) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	var args GeoRequest
	err := json.Unmarshal(b, &args)
	log.Printf("geo %v\n", args)
	res, err := st.geo.Nearby(&args)
	if err != nil {
		return 0, np.MkErrError(err)
	}
	st.rep, err = json.Marshal(res)
	if err != nil {
		return 0, np.MkErrError(err)
	}
	return np.Tsize(len(b)), nil
}

// XXX incremental read
func (st *Stream) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	if len(st.rep) == 0 || off > 0 {
		return nil, nil
	}
	return st.rep, nil
}

// Nearby returns all hotels within a given distance.
func (s *Geo) Nearby(req *GeoRequest) (*GeoResult, error) {
	var (
		points = s.getNearbyPoints(float64(req.Lat), float64(req.Lon))
		res    = &GeoResult{}
	)

	for _, p := range points {
		res.HotelIds = append(res.HotelIds, p.Id())
	}

	return res, nil
}

func (s *Geo) getNearbyPoints(lat, lon float64) []geoindex.Point {
	center := &geoindex.GeoPoint{
		Pid:  "",
		Plat: lat,
		Plon: lon,
	}

	return s.geoidx.KNearest(
		center,
		maxSearchResults,
		geoindex.Km(maxSearchRadius), func(p geoindex.Point) bool {
			return true
		},
	)
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

	return index
}
