package benchmarks

import (
// "time"
)

type HotelBenchConfig struct {
	NClients        int
	NCache          int
	NGeo            int
	NGeoIdx         int
	GeoSearchRadius int
	GeoNResults     int
	CacheMcpu       int
	ImgSzMB         int
	CacheAutoscale  bool
	UseMatch        bool
	Durs            string
	MaxRPS          string
	CacheType       string
	ScaleCache      *ManualScalingConfig
	ScaleGeo        *ManualScalingConfig
	ScaleCossim     *ManualScalingConfig
}
