package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/lmittmann/tint"
)

const envPrefix = "PLANESPOTTER"

// Config contains planespotter configuration loaded from the environment.
type Config struct {
	LogLevel               slog.Level    `split_words:"true" default:"INFO"`
	HTTPAddr               string        `split_words:"true" default:":8080"`
	Tar1090URL             string        `split_words:"true" required:"true"`
	MonitorInterval        time.Duration `split_words:"true" default:"15s"`
	MaxAltitude            int           `split_words:"true" default:"10000"`
	CallsignWaitReceives   int           `split_words:"true" default:"4"`
	DataPath               string        `split_words:"true" default:"."`
	CCAREnabled            bool          `split_words:"true" default:"true"`
	DiscordWebhookURL      string        `split_words:"true" required:"true"`
	DiscordWebhookThreadID string        `split_words:"true"`
	FlightAwareAPIKey      string        `split_words:"true"`
}

// Load loads .env, then populates Config from PLANESPOTTER_ environment variables.
func Load() (Config, error) {
	logLevel := new(slog.LevelVar)
	logLevel.Set(slog.LevelWarn)
	slog.SetDefault(
		slog.New(
			tint.NewHandler(os.Stderr, &tint.Options{
				Level: logLevel,
			}),
		),
	)

	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		slog.Warn("Error loading .env", "error", err)
	}

	var cfg Config
	if err := envconfig.Process(envPrefix, &cfg); err != nil {
		return Config{}, fmt.Errorf("process config: %w", err)
	}

	logLevel.Set(cfg.LogLevel)

	return cfg, nil
}
