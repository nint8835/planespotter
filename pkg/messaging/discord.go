package messaging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	adsbdb "github.com/nint8835/go-adsbdb"
	webhooks "github.com/typical-developers/discord-webhooks-go/v2"

	"github.com/nint8835/planespotter/pkg/ccar"
	"github.com/nint8835/planespotter/pkg/tar1090"
)

const (
	discordSendMaxAttempts = 3
	discordSendRetryDelay  = 5 * time.Second
)

// AircraftMessage contains the aircraft data used to build a Discord message.
type AircraftMessage struct {
	Aircraft          tar1090.Aircraft
	Details           *adsbdb.Aircraft
	CCAR              *ccar.Record
	Route             *adsbdb.FlightRoute
	Diversion         *DiversionInfo
	ImageURL          string
	ImageCopyright    string
	ImageCopyrightURL string
}

// NoopSender discards aircraft messages.
type NoopSender struct{}

// SendAircraft implements an aircraft sender that does nothing.
func (NoopSender) SendAircraft(context.Context, AircraftMessage) error {
	return nil
}

// DiscordSender sends aircraft messages to a Discord webhook.
type DiscordSender struct {
	client   *webhooks.WebhookClient
	threadID string
}

// NewDiscordSender creates a sender for the provided Discord webhook URL.
func NewDiscordSender(webhookURL string, threadID string) (*DiscordSender, error) {
	webhookURL = strings.TrimSpace(webhookURL)
	if webhookURL == "" {
		return nil, errors.New("discord webhook URL is required")
	}
	if _, err := url.ParseRequestURI(webhookURL); err != nil {
		return nil, fmt.Errorf("parse discord webhook URL: %w", err)
	}

	return &DiscordSender{
		client:   webhooks.NewWebhookClientFromURL(webhookURL),
		threadID: strings.TrimSpace(threadID),
	}, nil
}

// SendAircraft posts an aircraft notification to Discord.
func (s *DiscordSender) SendAircraft(ctx context.Context, message AircraftMessage) error {
	params := url.Values{}
	if s.threadID != "" {
		params.Set("thread_id", s.threadID)
	}

	payload := buildPayload(message)
	for attempt := 1; ; attempt++ {
		_, res, err := s.client.Execute(ctx, payload, &params)
		if res != nil && res.Body != nil {
			_ = res.Body.Close()
		}

		if err == nil && (res == nil || (res.StatusCode >= http.StatusOK && res.StatusCode < http.StatusMultipleChoices)) {
			return nil
		}

		wrappedErr := discordWebhookError(res, err)
		if !shouldRetryDiscordWebhook(res, err) || attempt >= discordSendMaxAttempts {
			return wrappedErr
		}

		delay := discordRetryDelay(res)
		slog.WarnContext(
			ctx,
			"Retrying Discord webhook send",
			"attempt", attempt,
			"max_attempts", discordSendMaxAttempts,
			"retry_delay", delay,
			"error", wrappedErr,
		)
		if err := waitForDiscordRetry(ctx, delay); err != nil {
			return err
		}
	}
}

func discordWebhookError(res *http.Response, err error) error {
	if err != nil {
		return fmt.Errorf("execute discord webhook: %w", err)
	}
	if res != nil && (res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices) {
		return fmt.Errorf("execute discord webhook: unexpected status %s", res.Status)
	}

	return nil
}

func shouldRetryDiscordWebhook(res *http.Response, err error) bool {
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}
		return true
	}
	if res == nil {
		return false
	}

	return res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= http.StatusInternalServerError
}

func discordRetryDelay(res *http.Response) time.Duration {
	if res == nil {
		return discordSendRetryDelay
	}
	delay, ok := parseRetryAfter(res.Header.Get("Retry-After"), time.Now())
	if ok {
		return delay
	}

	return discordSendRetryDelay
}

func parseRetryAfter(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}

	if seconds, err := time.ParseDuration(value + "s"); err == nil {
		if seconds < 0 {
			return 0, true
		}
		return seconds, true
	}

	retryAt, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	if delay := retryAt.Sub(now); delay > 0 {
		return delay, true
	}

	return 0, true
}

func waitForDiscordRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait to retry discord webhook: %w", ctx.Err())
	}
}

func buildPayload(message AircraftMessage) webhooks.MessagePayload {
	aircraft := message.Aircraft
	title := firstNonEmpty(
		strings.TrimSpace(aircraft.Flight),
		strings.TrimSpace(aircraft.Registration),
		strings.ToUpper(aircraft.Hex),
	)

	embed := webhooks.Embed{
		Author: &webhooks.EmbedAuthor{Name: authorName(message.Diversion)},
		Title:  title,
		URL:    flightInfoURL(aircraft),
		Color:  embedColor(aircraft, message.Diversion),
		Fields: fields(aircraft, message.Details, message.CCAR, message.Route, message.Diversion),
		Footer: footer(aircraft, message.Details),
	}
	if imageURL := aircraftImageURL(message.Details, message.ImageURL); imageURL != "" {
		if imageURL == strings.TrimSpace(message.ImageURL) {
			addPlanespottersImageField(&embed, message.ImageCopyright, message.ImageCopyrightURL)
		} else {
			embed.Image = &webhooks.EmbedImage{URL: imageURL}
		}
	}

	return webhooks.MessagePayload{
		Embeds: []webhooks.Embed{embed},
		AllowedMentions: &webhooks.AllowedMentions{
			Parse: []webhooks.AllowedMentionsParse{},
		},
	}
}

func addPlanespottersImageField(embed *webhooks.Embed, copyright string, copyrightURL string) {
	copyright = strings.TrimSpace(copyright)
	if copyright == "" {
		return
	}

	copyrightURL = strings.TrimSpace(copyrightURL)
	if copyrightURL != "" {
		copyright = fmt.Sprintf("[%s](%s)", markdownLinkText(copyright), markdownLinkURL(copyrightURL))
	}

	embed.Fields = append(embed.Fields, webhooks.EmbedField{
		Name:  "Planespotters.net image",
		Value: copyright,
	})
}

func markdownLinkText(text string) string {
	text = strings.ReplaceAll(text, `\`, `\\`)
	text = strings.ReplaceAll(text, "]", `\]`)
	return text
}

func markdownLinkURL(rawURL string) string {
	return strings.ReplaceAll(rawURL, ")", "%29")
}

func aircraftImageURL(details *adsbdb.Aircraft, fallback string) string {
	if details == nil || details.URLPhoto == nil {
		return strings.TrimSpace(fallback)
	}

	if photoURL := strings.TrimSpace(*details.URLPhoto); photoURL != "" {
		return photoURL
	}

	return strings.TrimSpace(fallback)
}

func flightInfoURL(aircraft tar1090.Aircraft) string {
	if aircraft.DBFlags&tar1090.DBFlagLADD != 0 {
		identifier := strings.TrimSpace(aircraft.Hex)
		if identifier == "" {
			return ""
		}

		return "https://globe.adsbexchange.com/?icao=" + url.QueryEscape(identifier)
	}

	identifier := firstNonEmpty(aircraft.Flight, aircraft.Registration)
	if identifier == "" {
		return ""
	}

	return "https://www.flightaware.com/live/flight/" + url.PathEscape(identifier)
}

func fields(
	aircraft tar1090.Aircraft,
	details *adsbdb.Aircraft,
	ccarRecord *ccar.Record,
	route *adsbdb.FlightRoute,
	diverting *DiversionInfo,
) []webhooks.EmbedField {
	var fields []webhooks.EmbedField
	addField := func(name string, value string) {
		if value = strings.TrimSpace(value); value != "" {
			fields = append(fields, webhooks.EmbedField{Name: name, Value: value})
		}
	}

	addField("Aircraft", identityLine(aircraft, details))
	addField("Operator", firstNonEmpty(airline(route), ccarOwner(ccarRecord), detailOwner(details), aircraft.OwnOp))
	addField("Route", routeDescription(route))
	addField("Possibly diverting", diverting.summary())
	if len(fields) == 0 {
		addField("Aircraft", "A previously unseen aircraft was picked up by tar1090.")
	}

	return fields
}

func footer(aircraft tar1090.Aircraft, details *adsbdb.Aircraft) *webhooks.EmbedFooter {
	text := strings.Join(nonEmptyValues(
		countryFlag(details),
		modeS(aircraft),
		dbFlagLabel(aircraft),
		detailCountry(details),
	), " · ")
	if text == "" {
		return nil
	}
	return &webhooks.EmbedFooter{Text: text}
}

// authorName labels a post by what is most worth knowing about it. A diversion is
// announced whether or not the aircraft is also a new spot.
func authorName(diverting *DiversionInfo) string {
	if diverting != nil {
		return "Aircraft possibly diverting"
	}
	return "New aircraft spotted"
}

func embedColor(aircraft tar1090.Aircraft, diverting *DiversionInfo) int {
	switch {
	case diverting != nil:
		return 0xf2994a
	case aircraft.DBFlags&tar1090.DBFlagMilitary != 0:
		return 0xeb5757
	case aircraft.DBFlags&tar1090.DBFlagInteresting != 0:
		return 0xf2c94c
	case aircraft.DBFlags&tar1090.DBFlagPIA != 0:
		return 0x9b51e0
	case aircraft.DBFlags&tar1090.DBFlagLADD != 0:
		return 0x828282
	default:
		return 0x2f80ed
	}
}
