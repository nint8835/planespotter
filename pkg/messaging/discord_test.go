package messaging_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	adsbdb "github.com/nint8835/go-adsbdb"

	"github.com/nint8835/planespotter/pkg/diversion"
	"github.com/nint8835/planespotter/pkg/flightaware"
	"github.com/nint8835/planespotter/pkg/messaging"
	"github.com/nint8835/planespotter/pkg/tar1090"
)

func TestDiscordSenderSendsAircraftMessageToThread(t *testing.T) {
	var gotThreadID string
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		gotThreadID = r.URL.Query().Get("thread_id")
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sender, err := messaging.NewDiscordSender(server.URL, "123456")
	if err != nil {
		t.Fatalf("NewDiscordSender() error = %v", err)
	}

	err = sender.SendAircraft(context.Background(), messaging.AircraftMessage{
		Aircraft: tar1090.Aircraft{
			Hex:          "c12345",
			Flight:       "ABC123",
			Registration: "C-GABC",
			AircraftType: "B738",
			Description:  "BOEING 737-800",
			DBFlags:      tar1090.DBFlagPIA | tar1090.DBFlagLADD,
		},
		Route: &adsbdb.FlightRoute{
			Airline: &adsbdb.Airline{Name: "Example Air"},
			Origin: adsbdb.Airport{
				IATACode:     "YYT",
				Municipality: "St. John's",
			},
			Destination: adsbdb.Airport{
				IATACode:     "YYZ",
				Municipality: "Toronto",
			},
		},
		Details: &adsbdb.Aircraft{RegisteredOwnerCountryISOName: "CA"},
	})
	if err != nil {
		t.Fatalf("SendAircraft() error = %v", err)
	}

	if gotThreadID != "123456" {
		t.Fatalf("thread_id = %q, want 123456", gotThreadID)
	}
	embeds, ok := gotPayload["embeds"].([]any)
	if !ok || len(embeds) != 1 {
		t.Fatalf("embeds = %#v, want one embed", gotPayload["embeds"])
	}
	embed, ok := embeds[0].(map[string]any)
	if !ok {
		t.Fatalf("embed = %#v, want object", embeds[0])
	}
	author, ok := embed["author"].(map[string]any)
	if !ok {
		t.Fatalf("author = %#v, want object", embed["author"])
	}
	if author["name"] != "New aircraft spotted" {
		t.Fatalf("author name = %#v, want new aircraft label", author["name"])
	}
	if embed["title"] != "ABC123" {
		t.Fatalf("embed title = %#v, want flight title", embed["title"])
	}
	if embed["url"] != "https://globe.adsbexchange.com/?icao=c12345" {
		t.Fatalf("embed url = %#v, want ADS-B Exchange hex link", embed["url"])
	}
	if _, ok := embed["description"]; ok {
		t.Fatalf("description = %#v, want no description", embed["description"])
	}
	fields, ok := embed["fields"].([]any)
	if !ok {
		t.Fatalf("fields = %#v, want list", embed["fields"])
	}
	assertEmbedField(t, fields, "Aircraft", "C-GABC · B738 (BOEING 737-800)")
	assertEmbedField(t, fields, "Operator", "Example Air")
	assertEmbedField(t, fields, "Route", "YYT (St. John's) -> YYZ (Toronto)")
	footer, ok := embed["footer"].(map[string]any)
	if !ok {
		t.Fatalf("footer = %#v, want object", embed["footer"])
	}
	if footer["text"] != "🇨🇦 · Mode S C12345 · PIA, LADD" {
		t.Fatalf("footer text = %#v, want flag, Mode S, and labels", footer["text"])
	}
	allowedMentions, ok := gotPayload["allowed_mentions"].(map[string]any)
	if !ok {
		t.Fatalf("allowed_mentions = %#v, want object", gotPayload["allowed_mentions"])
	}
	if len(allowedMentions) != 0 {
		t.Fatalf("allowed_mentions = %#v, want no mention parse values", allowedMentions)
	}
}

func TestDiscordSenderUsesCountryNameInFooterWhenFlagCodeIsMissing(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sender, err := messaging.NewDiscordSender(server.URL, "")
	if err != nil {
		t.Fatalf("NewDiscordSender() error = %v", err)
	}

	err = sender.SendAircraft(context.Background(), messaging.AircraftMessage{
		Aircraft: tar1090.Aircraft{Hex: "c12345"},
		Details:  &adsbdb.Aircraft{RegisteredOwnerCountryName: "Exampleland"},
	})
	if err != nil {
		t.Fatalf("SendAircraft() error = %v", err)
	}

	embeds := gotPayload["embeds"].([]any)
	embed := embeds[0].(map[string]any)
	footer := embed["footer"].(map[string]any)
	if footer["text"] != "Mode S C12345 · Exampleland" {
		t.Fatalf("footer text = %#v, want country fallback", footer["text"])
	}
}

func TestDiscordSenderLinksFlightInfoByRegistrationWithoutFlight(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sender, err := messaging.NewDiscordSender(server.URL, "")
	if err != nil {
		t.Fatalf("NewDiscordSender() error = %v", err)
	}

	err = sender.SendAircraft(context.Background(), messaging.AircraftMessage{
		Aircraft: tar1090.Aircraft{Hex: "c12345", Registration: "C-GABC"},
	})
	if err != nil {
		t.Fatalf("SendAircraft() error = %v", err)
	}

	embeds := gotPayload["embeds"].([]any)
	embed := embeds[0].(map[string]any)
	if embed["url"] != "https://www.flightaware.com/live/flight/C-GABC" {
		t.Fatalf("embed url = %#v, want FlightAware registration link", embed["url"])
	}
}

func TestDiscordSenderLinksLADDFlightInfoByHexWithoutFlight(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sender, err := messaging.NewDiscordSender(server.URL, "")
	if err != nil {
		t.Fatalf("NewDiscordSender() error = %v", err)
	}

	err = sender.SendAircraft(context.Background(), messaging.AircraftMessage{
		Aircraft: tar1090.Aircraft{
			Hex:          "c12345",
			Registration: "C-GABC",
			DBFlags:      tar1090.DBFlagLADD,
		},
	})
	if err != nil {
		t.Fatalf("SendAircraft() error = %v", err)
	}

	embeds := gotPayload["embeds"].([]any)
	embed := embeds[0].(map[string]any)
	if embed["url"] != "https://globe.adsbexchange.com/?icao=c12345" {
		t.Fatalf("embed url = %#v, want ADS-B Exchange hex link", embed["url"])
	}
}

func TestDiscordSenderUsesADSBDBPhoto(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sender, err := messaging.NewDiscordSender(server.URL, "")
	if err != nil {
		t.Fatalf("NewDiscordSender() error = %v", err)
	}

	photoURL := "https://example.test/photo.jpg"
	err = sender.SendAircraft(context.Background(), messaging.AircraftMessage{
		Aircraft: tar1090.Aircraft{Hex: "c12345"},
		Details:  &adsbdb.Aircraft{URLPhoto: &photoURL},
		ImageURL: "https://example.test/fallback.jpg",
	})
	if err != nil {
		t.Fatalf("SendAircraft() error = %v", err)
	}

	embeds := gotPayload["embeds"].([]any)
	embed := embeds[0].(map[string]any)
	image := embed["image"].(map[string]any)
	if image["url"] != photoURL {
		t.Fatalf("image url = %#v, want adsbdb photo", image["url"])
	}
}

func TestDiscordSenderUsesFallbackImageInsteadOfADSBDBThumbnail(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sender, err := messaging.NewDiscordSender(server.URL, "")
	if err != nil {
		t.Fatalf("NewDiscordSender() error = %v", err)
	}

	thumbnailURL := "https://example.test/thumb.jpg"
	err = sender.SendAircraft(context.Background(), messaging.AircraftMessage{
		Aircraft: tar1090.Aircraft{Hex: "c12345"},
		Details:  &adsbdb.Aircraft{URLPhotoThumbnail: &thumbnailURL},
		ImageURL: "https://example.test/fallback-large.jpg",
	})
	if err != nil {
		t.Fatalf("SendAircraft() error = %v", err)
	}

	embeds := gotPayload["embeds"].([]any)
	embed := embeds[0].(map[string]any)
	if _, ok := embed["image"]; ok {
		t.Fatalf("image = %#v, want omitted for fallback image", embed["image"])
	}
}

func TestDiscordSenderLinksFallbackImage(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sender, err := messaging.NewDiscordSender(server.URL, "")
	if err != nil {
		t.Fatalf("NewDiscordSender() error = %v", err)
	}

	err = sender.SendAircraft(context.Background(), messaging.AircraftMessage{
		Aircraft:          tar1090.Aircraft{Hex: "c12345"},
		ImageURL:          "https://example.test/fallback.jpg",
		ImageCopyright:    "Copyright © Example Photographer",
		ImageCopyrightURL: "https://www.planespotters.net/photo/123/example",
	})
	if err != nil {
		t.Fatalf("SendAircraft() error = %v", err)
	}

	embeds := gotPayload["embeds"].([]any)
	embed := embeds[0].(map[string]any)
	if _, ok := embed["image"]; ok {
		t.Fatalf("image = %#v, want omitted for fallback image", embed["image"])
	}
	fields := embed["fields"].([]any)
	assertEmbedField(
		t,
		fields,
		"Planespotters.net image",
		"[Copyright © Example Photographer](https://www.planespotters.net/photo/123/example)",
	)
}

func TestDiscordSenderOmitsTimestampForFooterWithoutCountry(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sender, err := messaging.NewDiscordSender(server.URL, "")
	if err != nil {
		t.Fatalf("NewDiscordSender() error = %v", err)
	}

	err = sender.SendAircraft(context.Background(), messaging.AircraftMessage{
		Aircraft: tar1090.Aircraft{Hex: "39d2aa"},
	})
	if err != nil {
		t.Fatalf("SendAircraft() error = %v", err)
	}

	embeds := gotPayload["embeds"].([]any)
	embed := embeds[0].(map[string]any)
	footer := embed["footer"].(map[string]any)
	if footer["text"] != "Mode S 39D2AA" {
		t.Fatalf("footer text = %#v, want only Mode S", footer["text"])
	}
	if _, ok := embed["timestamp"]; ok {
		t.Fatalf("timestamp = %#v, want omitted", embed["timestamp"])
	}
}

func TestDiscordSenderIncludesDatabaseFlagsInFooter(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sender, err := messaging.NewDiscordSender(server.URL, "")
	if err != nil {
		t.Fatalf("NewDiscordSender() error = %v", err)
	}

	err = sender.SendAircraft(context.Background(), messaging.AircraftMessage{
		Aircraft: tar1090.Aircraft{
			Hex:     "abc123",
			DBFlags: tar1090.DBFlagMilitary | tar1090.DBFlagInteresting,
		},
	})
	if err != nil {
		t.Fatalf("SendAircraft() error = %v", err)
	}

	embeds := gotPayload["embeds"].([]any)
	embed := embeds[0].(map[string]any)
	footer := embed["footer"].(map[string]any)
	if footer["text"] != "Mode S ABC123 · Military, Interesting" {
		t.Fatalf("footer text = %#v, want DB flag labels", footer["text"])
	}
}

func TestDiscordSenderColorsEmbedForDatabaseFlags(t *testing.T) {
	tests := []struct {
		name    string
		flags   int
		wantHex int
	}{
		{name: "default", flags: 0, wantHex: 0x2f80ed},
		{name: "military", flags: tar1090.DBFlagMilitary, wantHex: 0xeb5757},
		{name: "interesting", flags: tar1090.DBFlagInteresting, wantHex: 0xf2c94c},
		{name: "PIA", flags: tar1090.DBFlagPIA, wantHex: 0x9b51e0},
		{name: "LADD", flags: tar1090.DBFlagLADD, wantHex: 0x828282},
		{
			name:    "military takes priority",
			flags:   tar1090.DBFlagMilitary | tar1090.DBFlagPIA,
			wantHex: 0xeb5757,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPayload map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
					t.Errorf("decode payload: %v", err)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			sender, err := messaging.NewDiscordSender(server.URL, "")
			if err != nil {
				t.Fatalf("NewDiscordSender() error = %v", err)
			}

			err = sender.SendAircraft(context.Background(), messaging.AircraftMessage{
				Aircraft: tar1090.Aircraft{Hex: "abc123", DBFlags: tt.flags},
			})
			if err != nil {
				t.Fatalf("SendAircraft() error = %v", err)
			}

			embeds := gotPayload["embeds"].([]any)
			embed := embeds[0].(map[string]any)
			if embed["color"] != float64(tt.wantHex) {
				t.Fatalf("color = %#v, want %#x", embed["color"], tt.wantHex)
			}
		})
	}
}

func assertEmbedField(t *testing.T, fields []any, name string, value string) {
	t.Helper()

	for _, rawField := range fields {
		field, ok := rawField.(map[string]any)
		if !ok {
			t.Fatalf("field = %#v, want object", rawField)
		}
		if field["name"] == name {
			if field["value"] != value {
				t.Fatalf("%s field value = %#v, want %q", name, field["value"], value)
			}
			if _, ok := field["inline"]; ok {
				t.Fatalf("%s field inline = %#v, want omitted", name, field["inline"])
			}
			return
		}
	}

	t.Fatalf("missing %s field in %#v", name, fields)
}

func TestDiscordSenderReturnsErrorForNonSuccessResponse(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"bad webhook"}`))
	}))
	defer server.Close()

	sender, err := messaging.NewDiscordSender(server.URL, "")
	if err != nil {
		t.Fatalf("NewDiscordSender() error = %v", err)
	}

	err = sender.SendAircraft(context.Background(), messaging.AircraftMessage{
		Aircraft: tar1090.Aircraft{Hex: "c12345"},
	})
	if err == nil {
		t.Fatal("SendAircraft() error = nil, want error")
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestDiscordSenderRetriesRateLimitedResponse(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		if requests == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message":"rate limited"}`))
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sender, err := messaging.NewDiscordSender(server.URL, "")
	if err != nil {
		t.Fatalf("NewDiscordSender() error = %v", err)
	}

	err = sender.SendAircraft(context.Background(), messaging.AircraftMessage{
		Aircraft: tar1090.Aircraft{Hex: "c12345"},
	})
	if err != nil {
		t.Fatalf("SendAircraft() error = %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}

func TestDiscordSenderReturnsErrorAfterRetryableResponsesExhaustAttempts(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message":"rate limited"}`))
	}))
	defer server.Close()

	sender, err := messaging.NewDiscordSender(server.URL, "")
	if err != nil {
		t.Fatalf("NewDiscordSender() error = %v", err)
	}

	err = sender.SendAircraft(context.Background(), messaging.AircraftMessage{
		Aircraft: tar1090.Aircraft{Hex: "c12345"},
	})
	if err == nil {
		t.Fatal("SendAircraft() error = nil, want error")
	}
	if requests != 3 {
		t.Fatalf("requests = %d, want 3", requests)
	}
}

func TestDiscordSenderRendersDivertingAircraft(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sender, err := messaging.NewDiscordSender(server.URL, "")
	if err != nil {
		t.Fatalf("NewDiscordSender() error = %v", err)
	}

	err = sender.SendAircraft(context.Background(), messaging.AircraftMessage{
		Aircraft: tar1090.Aircraft{Hex: "c12345", Flight: "ABC123"},
		Diversion: messaging.GeometricDiversion(&diversion.Diversion{
			NearestAirport: adsbdb.Airport{IATACode: "YYZ", Municipality: "Toronto"},
			DistanceNM:     196,
			AltitudeFeet:   6000,
		}),
	})
	if err != nil {
		t.Fatalf("SendAircraft() error = %v", err)
	}

	embeds := gotPayload["embeds"].([]any)
	embed := embeds[0].(map[string]any)
	author := embed["author"].(map[string]any)
	if author["name"] != "Aircraft possibly diverting" {
		t.Fatalf("author name = %#v, want diversion label", author["name"])
	}
	if embed["color"] != float64(0xf2994a) {
		t.Fatalf("embed color = %#v, want diversion orange", embed["color"])
	}
	assertEmbedField(
		t,
		embed["fields"].([]any),
		"Possibly diverting",
		"At 6000 ft, 196 nm from the nearest airport on its filed route: YYZ (Toronto)",
	)
}

func TestDiscordSenderLabelsDivertingNewAircraftAsDiverting(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sender, err := messaging.NewDiscordSender(server.URL, "")
	if err != nil {
		t.Fatalf("NewDiscordSender() error = %v", err)
	}

	err = sender.SendAircraft(context.Background(), messaging.AircraftMessage{
		Aircraft:  tar1090.Aircraft{Hex: "c12345", Flight: "ABC123", DBFlags: tar1090.DBFlagMilitary},
		Diversion: messaging.GeometricDiversion(&diversion.Diversion{NearestAirport: adsbdb.Airport{IATACode: "YYZ"}}),
	})
	if err != nil {
		t.Fatalf("SendAircraft() error = %v", err)
	}

	embeds := gotPayload["embeds"].([]any)
	embed := embeds[0].(map[string]any)
	author := embed["author"].(map[string]any)
	if author["name"] != "Aircraft possibly diverting" {
		t.Fatalf("author name = %#v, want diversion label even for a new aircraft", author["name"])
	}
	if embed["color"] != float64(0xf2994a) {
		t.Fatalf("embed color = %#v, want diversion orange to outrank military red", embed["color"])
	}
}

func TestDiscordSenderOmitsDiversionFieldForAircraftNotDiverting(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sender, err := messaging.NewDiscordSender(server.URL, "")
	if err != nil {
		t.Fatalf("NewDiscordSender() error = %v", err)
	}

	err = sender.SendAircraft(context.Background(), messaging.AircraftMessage{
		Aircraft: tar1090.Aircraft{Hex: "c12345", Flight: "ABC123"},
	})
	if err != nil {
		t.Fatalf("SendAircraft() error = %v", err)
	}

	embeds := gotPayload["embeds"].([]any)
	embed := embeds[0].(map[string]any)
	if embed["color"] != float64(0x2f80ed) {
		t.Fatalf("embed color = %#v, want default blue", embed["color"])
	}
	for _, field := range embed["fields"].([]any) {
		if field.(map[string]any)["name"] == "Possibly diverting" {
			t.Fatalf("fields = %#v, want no diversion field", embed["fields"])
		}
	}
}

func TestDiscordSenderRendersFlightAwareDiversion(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sender, err := messaging.NewDiscordSender(server.URL, "")
	if err != nil {
		t.Fatalf("NewDiscordSender() error = %v", err)
	}

	err = sender.SendAircraft(context.Background(), messaging.AircraftMessage{
		Aircraft: tar1090.Aircraft{Hex: "c12345", Flight: "ACA123"},
		Diversion: messaging.FlightAwareDiversion(&flightaware.Flight{
			Diverted:    true,
			Origin:      &flightaware.Airport{CodeIATA: "YYT", City: "St. John's"},
			Destination: &flightaware.Airport{CodeIATA: "YYZ", City: "Toronto"},
		}),
	})
	if err != nil {
		t.Fatalf("SendAircraft() error = %v", err)
	}

	embeds := gotPayload["embeds"].([]any)
	embed := embeds[0].(map[string]any)
	if embed["color"] != float64(0xf2994a) {
		t.Fatalf("embed color = %#v, want diversion orange", embed["color"])
	}
	assertEmbedField(
		t,
		embed["fields"].([]any),
		"Possibly diverting",
		"FlightAware reports this flight as diverted, filed YYT (St. John's) -> YYZ (Toronto)",
	)
}

func TestFlightAwareDiversionIsNilWhenNotDiverted(t *testing.T) {
	if got := messaging.FlightAwareDiversion(&flightaware.Flight{Diverted: false}); got != nil {
		t.Fatalf("FlightAwareDiversion() = %#v, want nil", got)
	}
	if got := messaging.FlightAwareDiversion(nil); got != nil {
		t.Fatalf("FlightAwareDiversion(nil) = %#v, want nil", got)
	}
}
