package flightaware

// flightsResponse is the top-level GET /flights/{ident} response.
type flightsResponse struct {
	Flights  []Flight `json:"flights"`
	NumPages int      `json:"num_pages"`
}

// Flight is the subset of an AeroAPI flight object used to report diversions.
type Flight struct {
	Ident        string   `json:"ident"`
	Registration string   `json:"registration"`
	Diverted     bool     `json:"diverted"`
	Cancelled    bool     `json:"cancelled"`
	Origin       *Airport `json:"origin"`
	Destination  *Airport `json:"destination"`
	// ActualOff and ActualOn are the actual runway departure and arrival times, in
	// ISO8601. They are empty until the event happens, so a flight that has taken
	// off but not landed is the one currently airborne.
	ActualOff string `json:"actual_off"`
	ActualOn  string `json:"actual_on"`
}

// Airborne reports whether the flight has departed but not yet arrived.
func (f Flight) Airborne() bool {
	return f.ActualOff != "" && f.ActualOn == ""
}

// Airport is the subset of an AeroAPI FlightAirportRef used to name a route.
type Airport struct {
	Code     string `json:"code"`
	CodeICAO string `json:"code_icao"`
	CodeIATA string `json:"code_iata"`
	Name     string `json:"name"`
	City     string `json:"city"`
}

// Label returns the most human-friendly identifier available for the airport.
func (a *Airport) Label() string {
	if a == nil {
		return ""
	}
	code := firstNonEmpty(a.CodeIATA, a.CodeICAO, a.Code)
	switch {
	case code != "" && a.City != "":
		return code + " (" + a.City + ")"
	case code != "":
		return code
	default:
		return a.Name
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
