package messaging

import (
	"fmt"
	"strings"

	adsbdb "github.com/nint8835/go-adsbdb"

	"github.com/nint8835/planespotter/pkg/ccar"
	"github.com/nint8835/planespotter/pkg/diversion"
	"github.com/nint8835/planespotter/pkg/flightaware"
	"github.com/nint8835/planespotter/pkg/tar1090"
)

func routeDescription(route *adsbdb.FlightRoute) string {
	if route == nil {
		return ""
	}

	airports := []string{airport(route.Origin)}
	if route.Midpoint != nil {
		airports = append(airports, airport(*route.Midpoint))
	}
	airports = append(airports, airport(route.Destination))

	return strings.Join(airports, " -> ")
}

func airport(airport adsbdb.Airport) string {
	code := firstNonEmpty(airport.IATACode, airport.ICAOCode)
	if code == "" {
		return airport.Name
	}
	if airport.Municipality == "" {
		return code
	}
	return fmt.Sprintf("%s (%s)", code, airport.Municipality)
}

// DiversionInfo is a rendered diversion ready to show in a Discord embed. It is
// source-agnostic: the geometric heuristic and FlightAware each produce one.
type DiversionInfo struct {
	Summary string
}

func (d *DiversionInfo) summary() string {
	if d == nil {
		return ""
	}
	return d.Summary
}

// GeometricDiversion renders a diversion inferred from an aircraft's position
// relative to its filed route, or returns nil when there is none.
func GeometricDiversion(diverting *diversion.Diversion) *DiversionInfo {
	if diverting == nil {
		return nil
	}

	return &DiversionInfo{
		Summary: fmt.Sprintf(
			"At %d ft, %.0f nm from the nearest airport on its filed route: %s",
			diverting.AltitudeFeet,
			diverting.DistanceNM,
			airport(diverting.NearestAirport),
		),
	}
}

// FlightAwareDiversion renders a diversion reported by FlightAware, or returns nil
// when the flight is missing or not diverted.
func FlightAwareDiversion(flight *flightaware.Flight) *DiversionInfo {
	if flight == nil || !flight.Diverted {
		return nil
	}

	summary := "FlightAware reports this flight as diverted"
	if filed := flightawareRoute(flight); filed != "" {
		summary += ", filed " + filed
	}

	return &DiversionInfo{Summary: summary}
}

func flightawareRoute(flight *flightaware.Flight) string {
	origin := flight.Origin.Label()
	destination := flight.Destination.Label()
	switch {
	case origin != "" && destination != "":
		return origin + " -> " + destination
	case destination != "":
		return "to " + destination
	case origin != "":
		return "from " + origin
	default:
		return ""
	}
}

func airline(route *adsbdb.FlightRoute) string {
	if route == nil || route.Airline == nil {
		return ""
	}
	return route.Airline.Name
}

func detailRegistration(details *adsbdb.Aircraft) string {
	if details == nil {
		return ""
	}
	return details.Registration
}

func detailType(details *adsbdb.Aircraft) string {
	if details == nil {
		return ""
	}
	return firstNonEmpty(details.ICAOType, details.Type, details.Manufacturer)
}

func detailOwner(details *adsbdb.Aircraft) string {
	if details == nil {
		return ""
	}
	return details.RegisteredOwner
}

func ccarOwner(record *ccar.Record) string {
	if record == nil {
		return ""
	}
	return record.OwnerName()
}

func detailCountry(details *adsbdb.Aircraft) string {
	if details == nil || countryFlag(details) != "" {
		return ""
	}
	return details.RegisteredOwnerCountryName
}

func countryFlag(details *adsbdb.Aircraft) string {
	if details == nil {
		return ""
	}

	code := strings.ToUpper(strings.TrimSpace(details.RegisteredOwnerCountryISOName))
	if len(code) != 2 || code[0] < 'A' || code[0] > 'Z' || code[1] < 'A' || code[1] > 'Z' {
		return ""
	}

	return string([]rune{
		regionalIndicator(code[0]),
		regionalIndicator(code[1]),
	})
}

func regionalIndicator(letter byte) rune {
	return 0x1F1E6 + rune(letter-'A')
}

func modeS(aircraft tar1090.Aircraft) string {
	hex := strings.ToUpper(strings.TrimSpace(aircraft.Hex))
	if hex == "" {
		return ""
	}
	return "Mode S " + hex
}

func dbFlagLabel(aircraft tar1090.Aircraft) string {
	var labels []string
	if aircraft.DBFlags&tar1090.DBFlagMilitary != 0 {
		labels = append(labels, "Military")
	}
	if aircraft.DBFlags&tar1090.DBFlagInteresting != 0 {
		labels = append(labels, "Interesting")
	}
	if aircraft.DBFlags&tar1090.DBFlagPIA != 0 {
		labels = append(labels, "PIA")
	}
	if aircraft.DBFlags&tar1090.DBFlagLADD != 0 {
		labels = append(labels, "LADD")
	}
	return strings.Join(labels, ", ")
}

func identityLine(aircraft tar1090.Aircraft, details *adsbdb.Aircraft) string {
	return strings.Join(nonEmptyValues(
		firstNonEmpty(aircraft.Registration, detailRegistration(details)),
		typeDescription(aircraft, details),
	), " · ")
}

func typeDescription(aircraft tar1090.Aircraft, details *adsbdb.Aircraft) string {
	aircraftType := firstNonEmpty(aircraft.AircraftType, detailType(details))
	if aircraft.Description == "" {
		return aircraftType
	}
	if aircraftType == "" {
		return aircraft.Description
	}
	return fmt.Sprintf("%s (%s)", aircraftType, aircraft.Description)
}

func nonEmptyValues(values ...string) []string {
	var nonEmpty []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			nonEmpty = append(nonEmpty, value)
		}
	}
	return nonEmpty
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
