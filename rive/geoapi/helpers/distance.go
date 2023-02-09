package helpers

import (
	"dmpsupport/rive/geoapi"
	"math"
)

func GPSDistance(pos1, pos2 geoapi.GPS) float64 {
	var earth float64 = 6378.137 //km
	lat1 := pos1.Latitude * math.Pi / 180
	lat2 := pos2.Latitude * math.Pi / 180

	dlat := (pos2.Latitude - pos1.Latitude) * math.Pi / 180
	dlon := (pos2.Longitude - pos1.Longitude) * math.Pi / 180

	a := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Sin(dlon/2)*math.Sin(dlon/2)*math.Cos(lat1)*math.Cos(lat2)
	b := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earth * b
}
