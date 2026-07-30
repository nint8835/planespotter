package flightaware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nint8835/planespotter/pkg/flightaware"
)

func TestClientRequiresAPIKey(t *testing.T) {
	if _, err := flightaware.NewClient(""); err == nil {
		t.Fatal("NewClient(\"\") error = nil, want error")
	}
}

func TestCurrentFlightSendsAuthenticatedRequest(t *testing.T) {
	var gotPath string
	var gotIdentType string
	var gotAPIKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotIdentType = r.URL.Query().Get("ident_type")
		gotAPIKey = r.Header.Get("x-apikey")
		_, _ = w.Write([]byte(`{"num_pages":1,"flights":[
			{"ident":"ACA123","registration":"C-GABC","diverted":true,
			 "actual_off":"2026-07-30T12:00:00Z","actual_on":"",
			 "origin":{"code_iata":"YYT","city":"St. John's"},
			 "destination":{"code_iata":"YYZ","city":"Toronto"}}
		]}`))
	}))
	defer server.Close()

	client, err := flightaware.NewClient("secret-key", flightaware.WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	flight, err := client.CurrentFlight(context.Background(), "C-GABC", flightaware.IdentTypeRegistration)
	if err != nil {
		t.Fatalf("CurrentFlight() error = %v", err)
	}

	if gotAPIKey != "secret-key" {
		t.Errorf("x-apikey header = %q, want secret-key", gotAPIKey)
	}
	if gotPath != "/flights/C-GABC" {
		t.Errorf("request path = %q, want /flights/C-GABC", gotPath)
	}
	if gotIdentType != "registration" {
		t.Errorf("ident_type = %q, want registration", gotIdentType)
	}
	if flight == nil {
		t.Fatal("CurrentFlight() = nil, want airborne flight")
	}
	if !flight.Diverted {
		t.Error("flight.Diverted = false, want true")
	}
	if flight.Destination.Label() != "YYZ (Toronto)" {
		t.Errorf("destination label = %q, want YYZ (Toronto)", flight.Destination.Label())
	}
}

// A flight that has already landed is not the one an aircraft in the air is on.
func TestCurrentFlightIgnoresFlightsThatHaveArrived(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"num_pages":1,"flights":[
			{"ident":"ACA123","diverted":true,
			 "actual_off":"2026-07-30T10:00:00Z","actual_on":"2026-07-30T11:00:00Z"}
		]}`))
	}))
	defer server.Close()

	client, err := flightaware.NewClient("secret-key", flightaware.WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	flight, err := client.CurrentFlight(context.Background(), "ACA123", flightaware.IdentTypeDesignator)
	if err != nil {
		t.Fatalf("CurrentFlight() error = %v", err)
	}
	if flight != nil {
		t.Fatalf("CurrentFlight() = %#v, want nil for an arrived flight", flight)
	}
}

func TestFlightsReturnsErrorOnUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"title":"unauthorized"}`))
	}))
	defer server.Close()

	client, err := flightaware.NewClient("bad-key", flightaware.WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if _, err := client.Flights(context.Background(), "ACA123", flightaware.IdentTypeDesignator); err == nil {
		t.Fatal("Flights() error = nil, want error on 401")
	}
}
