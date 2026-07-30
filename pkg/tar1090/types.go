package tar1090

import (
	"encoding/json"
	"fmt"
)

// AircraftResponse is the top-level /data/aircraft.json response.
type AircraftResponse struct {
	Now      float64    `json:"now"`
	Messages int64      `json:"messages"`
	Aircraft []Aircraft `json:"aircraft"`
}

// Aircraft contains the common aircraft fields returned by tar1090.
type Aircraft struct {
	Hex               string             `json:"hex"`
	Type              string             `json:"type,omitempty"`
	Flight            string             `json:"flight,omitempty"`
	Registration      string             `json:"r,omitempty"`
	AircraftType      string             `json:"t,omitempty"`
	Description       string             `json:"desc,omitempty"`
	OwnOp             string             `json:"ownOp,omitempty"`
	Year              string             `json:"year,omitempty"`
	DBFlags           int                `json:"dbFlags,omitempty"`
	AltitudeBaro      BarometricAltitude `json:"alt_baro"`
	AltitudeGeom      *int               `json:"alt_geom,omitempty"`
	GroundSpeed       *float64           `json:"gs,omitempty"`
	IndicatedAirspeed *int               `json:"ias,omitempty"`
	TrueAirspeed      *int               `json:"tas,omitempty"`
	Mach              *float64           `json:"mach,omitempty"`
	WindDirection     *int               `json:"wd,omitempty"`
	WindSpeed         *int               `json:"ws,omitempty"`
	OutsideAirTemp    *int               `json:"oat,omitempty"`
	TotalAirTemp      *int               `json:"tat,omitempty"`
	Track             *float64           `json:"track,omitempty"`
	TrackRate         *float64           `json:"track_rate,omitempty"`
	Roll              *float64           `json:"roll,omitempty"`
	MagHeading        *float64           `json:"mag_heading,omitempty"`
	TrueHeading       *float64           `json:"true_heading,omitempty"`
	BaroRate          *int               `json:"baro_rate,omitempty"`
	GeomRate          *int               `json:"geom_rate,omitempty"`
	Squawk            string             `json:"squawk,omitempty"`
	Emergency         string             `json:"emergency,omitempty"`
	Category          string             `json:"category,omitempty"`
	NavQNH            *float64           `json:"nav_qnh,omitempty"`
	NavAltitudeMCP    *int               `json:"nav_altitude_mcp,omitempty"`
	NavHeading        *float64           `json:"nav_heading,omitempty"`
	NavModes          []string           `json:"nav_modes,omitempty"`
	Latitude          *float64           `json:"lat,omitempty"`
	Longitude         *float64           `json:"lon,omitempty"`
	LastPosition      *Position          `json:"lastPosition,omitempty"`
	NIC               *int               `json:"nic,omitempty"`
	RC                *int               `json:"rc,omitempty"`
	SeenPosition      *float64           `json:"seen_pos,omitempty"`
	ReceiverDistance  *float64           `json:"r_dst,omitempty"`
	ReceiverBearing   *float64           `json:"r_dir,omitempty"`
	Version           *int               `json:"version,omitempty"`
	NICBaro           *int               `json:"nic_baro,omitempty"`
	NACP              *int               `json:"nac_p,omitempty"`
	NACV              *int               `json:"nac_v,omitempty"`
	SIL               *int               `json:"sil,omitempty"`
	SILType           string             `json:"sil_type,omitempty"`
	GVA               *int               `json:"gva,omitempty"`
	SDA               *int               `json:"sda,omitempty"`
	Alert             *int               `json:"alert,omitempty"`
	SPI               *int               `json:"spi,omitempty"`
	MLAT              []string           `json:"mlat,omitempty"`
	TISB              []string           `json:"tisb,omitempty"`
	Messages          int64              `json:"messages"`
	Seen              *float64           `json:"seen,omitempty"`
	RSSI              *float64           `json:"rssi,omitempty"`
}

// Altitude returns the aircraft's altitude in feet, preferring the barometric
// value and falling back to the geometric one. It reports false when the aircraft
// is on the ground or no altitude is known.
func (a Aircraft) Altitude() (int, bool) {
	if a.AltitudeBaro.Ground {
		return 0, false
	}
	if a.AltitudeBaro.Feet != nil {
		return *a.AltitudeBaro.Feet, true
	}
	if a.AltitudeGeom != nil {
		return *a.AltitudeGeom, true
	}
	return 0, false
}

// Position returns the aircraft's latitude and longitude, preferring its live
// position and falling back to its last known one. It reports false when neither
// is known.
func (a Aircraft) Position() (float64, float64, bool) {
	if a.Latitude != nil && a.Longitude != nil {
		return *a.Latitude, *a.Longitude, true
	}
	if a.LastPosition != nil {
		return a.LastPosition.Latitude, a.LastPosition.Longitude, true
	}
	return 0, 0, false
}

// Database flags from readsb/tar1090's aircraft database.
const (
	DBFlagMilitary = 1 << iota
	DBFlagInteresting
	DBFlagPIA
	DBFlagLADD
)

// Position describes an aircraft position, including stale lastPosition values.
type Position struct {
	Latitude     float64  `json:"lat"`
	Longitude    float64  `json:"lon"`
	NIC          *int     `json:"nic,omitempty"`
	RC           *int     `json:"rc,omitempty"`
	SeenPosition *float64 `json:"seen_pos,omitempty"`
}

// BarometricAltitude is either a numeric altitude in feet or the string "ground".
type BarometricAltitude struct {
	Feet   *int
	Ground bool
}

// UnmarshalJSON decodes tar1090's alt_baro field, which may be a number or "ground".
func (a *BarometricAltitude) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	var feet int
	if err := json.Unmarshal(data, &feet); err == nil {
		a.Feet = &feet
		a.Ground = false
		return nil
	}

	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("decode barometric altitude: %w", err)
	}
	if value != "ground" {
		return fmt.Errorf("decode barometric altitude: unsupported value %q", value)
	}

	a.Feet = nil
	a.Ground = true
	return nil
}
