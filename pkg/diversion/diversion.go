// Package diversion detects aircraft that appear to be landing somewhere other
// than the airports on their filed route.
//
// Callers are expected to have already narrowed aircraft to the altitudes they
// care about, and that ceiling is what gives the check its meaning. A normal
// flight is only low near an airport it filed for: it is still climbing within
// tens of miles of its origin, and it does not leave cruise until it is close to
// its destination. So being low and far from every filed airport is itself the
// anomaly, whether the aircraft is still descending or has already levelled off
// to cruise to wherever it is actually going.
package diversion

import (
	"math"

	adsbdb "github.com/nint8835/go-adsbdb"

	"github.com/nint8835/planespotter/pkg/tar1090"
)

const (
	// AirportRadiusNM is the distance within which a low aircraft is treated as
	// arriving at or departing a filed airport rather than diverting away from it.
	AirportRadiusNM = 75

	earthRadiusNM = 3440.065
)

// Diversion describes an aircraft that is low and far from every airport on its
// filed route.
type Diversion struct {
	NearestAirport adsbdb.Airport
	DistanceNM     float64
	AltitudeFeet   int
}

// Detect describes how an aircraft appears to be diverting, or returns nil if it
// does not. An aircraft low enough to be near a landing, but far from every airport
// on its filed route, is going somewhere it did not file for.
//
// Vertical rate is deliberately not considered. A diverting aircraft descends to a
// new altitude and then holds it, often for a hundred miles, so requiring a descent
// would miss it for most of the diversion.
func Detect(aircraft tar1090.Aircraft, route *adsbdb.FlightRoute) *Diversion {
	if route == nil {
		return nil
	}

	feet, ok := aircraft.Altitude()
	if !ok {
		return nil
	}
	latitude, longitude, ok := aircraft.Position()
	if !ok {
		return nil
	}
	nearest, distance, ok := nearestFiledAirport(route, latitude, longitude)
	if !ok || distance <= AirportRadiusNM {
		return nil
	}

	return &Diversion{
		NearestAirport: nearest,
		DistanceNM:     distance,
		AltitudeFeet:   feet,
	}
}

func nearestFiledAirport(
	route *adsbdb.FlightRoute,
	latitude float64,
	longitude float64,
) (adsbdb.Airport, float64, bool) {
	filed := []adsbdb.Airport{route.Origin, route.Destination}
	if route.Midpoint != nil {
		filed = append(filed, *route.Midpoint)
	}

	var nearest adsbdb.Airport
	nearestDistance := math.Inf(1)
	found := false
	for _, candidate := range filed {
		// A zero position is adsbdb having no coordinates for the airport rather than
		// a real location on the null island.
		if candidate.Latitude == 0 && candidate.Longitude == 0 {
			continue
		}

		distance := distanceNM(latitude, longitude, candidate.Latitude, candidate.Longitude)
		if distance < nearestDistance {
			nearest, nearestDistance, found = candidate, distance, true
		}
	}

	return nearest, nearestDistance, found
}

func distanceNM(latitude1 float64, longitude1 float64, latitude2 float64, longitude2 float64) float64 {
	latitude1Rad := degreesToRadians(latitude1)
	latitude2Rad := degreesToRadians(latitude2)
	deltaLatitude := degreesToRadians(latitude2 - latitude1)
	deltaLongitude := degreesToRadians(longitude2 - longitude1)

	haversine := math.Sin(deltaLatitude/2)*math.Sin(deltaLatitude/2) +
		math.Cos(latitude1Rad)*math.Cos(latitude2Rad)*
			math.Sin(deltaLongitude/2)*math.Sin(deltaLongitude/2)

	return 2 * earthRadiusNM * math.Asin(math.Sqrt(haversine))
}

func degreesToRadians(degrees float64) float64 {
	return degrees * math.Pi / 180
}
