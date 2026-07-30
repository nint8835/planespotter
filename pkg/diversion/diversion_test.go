package diversion_test

import (
	"testing"

	adsbdb "github.com/nint8835/go-adsbdb"

	"github.com/nint8835/planespotter/pkg/diversion"
	"github.com/nint8835/planespotter/pkg/tar1090"
)

func torontoRoute() *adsbdb.FlightRoute {
	return &adsbdb.FlightRoute{
		Origin: adsbdb.Airport{
			IATACode:  "YYT",
			Latitude:  47.6187,
			Longitude: -52.7519,
		},
		Destination: adsbdb.Airport{
			IATACode:     "YYZ",
			Municipality: "Toronto",
			Latitude:     43.6777,
			Longitude:    -79.6248,
		},
	}
}

// Low over Ottawa, roughly 196 nm from Toronto, the filed destination.
func lowOverOttawa() tar1090.Aircraft {
	return tar1090.Aircraft{
		Hex:          "c12345",
		Flight:       "ABC123",
		AltitudeBaro: tar1090.BarometricAltitude{Feet: new(6000)},
		BaroRate:     new(-1200),
		Latitude:     new(45.3225),
		Longitude:    new(-75.6692),
	}
}

func TestDetectFlagsAircraftLowFarFromEveryFiledAirport(t *testing.T) {
	got := diversion.Detect(lowOverOttawa(), torontoRoute())
	if got == nil {
		t.Fatal("Detect() = nil, want diversion")
	}
	if got.NearestAirport.IATACode != "YYZ" {
		t.Errorf("nearest airport = %q, want YYZ", got.NearestAirport.IATACode)
	}
	if got.DistanceNM < 190 || got.DistanceNM > 200 {
		t.Errorf("distance = %v nm, want roughly 196 nm", got.DistanceNM)
	}
	if got.AltitudeFeet != 6000 {
		t.Errorf("altitude = %v, want 6000", got.AltitudeFeet)
	}
}

func TestDetectIgnoresAircraftNearFiledDestination(t *testing.T) {
	aircraft := lowOverOttawa()
	// Near Toronto, its filed destination.
	aircraft.Latitude = new(43.8)
	aircraft.Longitude = new(-79.7)

	if got := diversion.Detect(aircraft, torontoRoute()); got != nil {
		t.Fatalf("Detect() = %#v, want nil", got)
	}
}

func TestDetectIgnoresAircraftNearFiledMidpoint(t *testing.T) {
	route := torontoRoute()
	route.Midpoint = &adsbdb.Airport{
		IATACode:  "YOW",
		Latitude:  45.3225,
		Longitude: -75.6692,
	}

	if got := diversion.Detect(lowOverOttawa(), route); got != nil {
		t.Fatalf("Detect() = %#v, want nil", got)
	}
}

// A diverting aircraft levels off and cruises to wherever it is actually going, so
// it is not descending for most of the diversion.
func TestDetectFlagsAircraftLevelFarFromEveryFiledAirport(t *testing.T) {
	aircraft := lowOverOttawa()
	aircraft.BaroRate = new(0)

	if got := diversion.Detect(aircraft, torontoRoute()); got == nil {
		t.Fatal("Detect() = nil, want a levelled-off diversion")
	}
}

func TestDetectFlagsAircraftWithoutVerticalRate(t *testing.T) {
	aircraft := lowOverOttawa()
	aircraft.BaroRate = nil
	aircraft.GeomRate = nil

	if got := diversion.Detect(aircraft, torontoRoute()); got == nil {
		t.Fatal("Detect() = nil, want diversion")
	}
}

func TestDetectIgnoresAircraftWithoutAltitude(t *testing.T) {
	aircraft := lowOverOttawa()
	aircraft.AltitudeBaro = tar1090.BarometricAltitude{}

	if got := diversion.Detect(aircraft, torontoRoute()); got != nil {
		t.Fatalf("Detect() = %#v, want nil", got)
	}
}

func TestDetectIgnoresAircraftWithoutFiledRoute(t *testing.T) {
	if got := diversion.Detect(lowOverOttawa(), nil); got != nil {
		t.Fatalf("Detect() = %#v, want nil", got)
	}
}

func TestDetectIgnoresAircraftWithoutPosition(t *testing.T) {
	aircraft := lowOverOttawa()
	aircraft.Latitude = nil
	aircraft.Longitude = nil

	if got := diversion.Detect(aircraft, torontoRoute()); got != nil {
		t.Fatalf("Detect() = %#v, want nil", got)
	}
}

func TestDetectFallsBackToLastKnownPosition(t *testing.T) {
	aircraft := lowOverOttawa()
	aircraft.Latitude = nil
	aircraft.Longitude = nil
	aircraft.LastPosition = &tar1090.Position{Latitude: 45.3225, Longitude: -75.6692}

	if got := diversion.Detect(aircraft, torontoRoute()); got == nil {
		t.Fatal("Detect() = nil, want diversion")
	}
}

// adsbdb returns a zero position for airports it has no coordinates for, which
// would otherwise put them in the Gulf of Guinea and look like a diversion.
func TestDetectIgnoresFiledAirportsWithoutCoordinates(t *testing.T) {
	route := &adsbdb.FlightRoute{
		Origin:      adsbdb.Airport{IATACode: "YYT"},
		Destination: adsbdb.Airport{IATACode: "YYZ"},
	}

	if got := diversion.Detect(lowOverOttawa(), route); got != nil {
		t.Fatalf("Detect() = %#v, want nil", got)
	}
}

func TestDetectUsesGeometricAltitudeWhenBarometricIsMissing(t *testing.T) {
	aircraft := lowOverOttawa()
	aircraft.AltitudeBaro = tar1090.BarometricAltitude{}
	aircraft.AltitudeGeom = new(6000)

	if got := diversion.Detect(aircraft, torontoRoute()); got == nil {
		t.Fatal("Detect() = nil, want diversion")
	}
}

func TestDetectIgnoresAircraftOnGround(t *testing.T) {
	aircraft := lowOverOttawa()
	aircraft.AltitudeBaro = tar1090.BarometricAltitude{Ground: true}

	if got := diversion.Detect(aircraft, torontoRoute()); got != nil {
		t.Fatalf("Detect() = %#v, want nil", got)
	}
}
