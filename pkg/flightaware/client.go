// Package flightaware queries FlightAware's AeroAPI for authoritative flight
// status, including whether a flight has been diverted.
package flightaware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const defaultBaseURL = "https://aeroapi.flightaware.com/aeroapi"

// IdentType selects how AeroAPI interprets an ident.
type IdentType string

const (
	// IdentTypeRegistration looks a flight up by aircraft registration.
	IdentTypeRegistration IdentType = "registration"
	// IdentTypeDesignator looks a flight up by flight designator (callsign).
	IdentTypeDesignator IdentType = "designator"
)

// Client queries the AeroAPI flights endpoint.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	apiKey     string
	userAgent  string
}

// Option configures a Client.
type Option func(*Client) error

// WithBaseURL overrides the AeroAPI base URL, primarily for testing.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) error {
		parsedURL, err := url.Parse(baseURL)
		if err != nil {
			return fmt.Errorf("parse flightaware base URL: %w", err)
		}
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			return fmt.Errorf("flightaware base URL must use http or https")
		}
		if parsedURL.Host == "" {
			return fmt.Errorf("flightaware base URL must include a host")
		}

		c.baseURL = parsedURL
		return nil
	}
}

// WithHTTPClient configures the HTTP client used for API requests.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) error {
		if httpClient == nil {
			return fmt.Errorf("http client is nil")
		}

		c.httpClient = httpClient
		return nil
	}
}

// WithUserAgent configures an optional User-Agent header for API requests.
func WithUserAgent(userAgent string) Option {
	return func(c *Client) error {
		c.userAgent = strings.TrimSpace(userAgent)
		return nil
	}
}

// NewClient creates an AeroAPI client authenticated with the given API key.
func NewClient(apiKey string, opts ...Option) (*Client, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("flightaware api key is required")
	}

	baseURL, err := url.Parse(defaultBaseURL)
	if err != nil {
		panic(err)
	}

	client := &Client{
		baseURL:    baseURL,
		httpClient: http.DefaultClient,
		apiKey:     apiKey,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(client); err != nil {
			return nil, err
		}
	}

	return client, nil
}

// Flights returns the flights AeroAPI knows for an ident, most recent first.
func (c *Client) Flights(ctx context.Context, ident string, identType IdentType) ([]Flight, error) {
	if c == nil {
		return nil, fmt.Errorf("flightaware client is nil")
	}
	ident = strings.TrimSpace(ident)
	if ident == "" {
		return nil, fmt.Errorf("ident is required")
	}

	flightsPath := strings.TrimRight(c.baseURL.Path, "/") + "/flights/" + url.PathEscape(ident)
	requestURL := c.baseURL.ResolveReference(&url.URL{Path: flightsPath})
	query := requestURL.Query()
	if identType != "" {
		query.Set("ident_type", string(identType))
	}
	requestURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create flightaware request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-apikey", c.apiKey)
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch flightaware flights: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf(
			"fetch flightaware flights: unexpected status %s: %s",
			resp.Status,
			strings.TrimSpace(string(body)),
		)
	}

	var flightsResponse flightsResponse
	if err := json.NewDecoder(resp.Body).Decode(&flightsResponse); err != nil {
		return nil, fmt.Errorf("decode flightaware flights response: %w", err)
	}

	return flightsResponse.Flights, nil
}

// CurrentFlight returns the flight the ident is currently operating, or nil if
// AeroAPI knows of no airborne flight for it. The airborne flight is the one an
// aircraft being received over ADS-B is on right now.
func (c *Client) CurrentFlight(ctx context.Context, ident string, identType IdentType) (*Flight, error) {
	flights, err := c.Flights(ctx, ident, identType)
	if err != nil {
		return nil, err
	}

	for _, flight := range flights {
		if flight.Airborne() {
			return &flight, nil
		}
	}

	return nil, nil
}
