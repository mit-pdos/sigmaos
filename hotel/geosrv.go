package hotel

import (
	"encoding/json"
	"log"
	"strconv"

	"github.com/hailocab/go-geoindex"
	"github.com/harlow/go-micro-services/data"
	// "github.com/harlow/go-micro-services/internal/proto/geo"

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
func RunGeoSrv(n string) error {
	geo := &Geo{}
	geo.geoidx = newGeoIndex("data/geo.json")
	pds := protdevsrv.MakeProtDevSrv(np.HOTELGEO, geo)
	return pds.RunServer()
}

// Nearby returns all hotels within a given distance.
func (s *Geo) Nearby(req GeoRequest, rep *GeoResult) error {
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
	for i := 7; i < NHOTEL; i++ {
		p := &geoindex.GeoPoint{
			Pid:  strconv.Itoa(i),
			Plat: 37.7835 + float64(i)/500.0*3,
			Plon: -122.41 + float64(i)/500.0*4,
		}
		index.Add(p)
	}
	return index
}
