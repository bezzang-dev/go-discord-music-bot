package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultLogLevel                     = "info"
	defaultMetricsAddr                  = "127.0.0.1:2112"
	defaultMetricsLavalinkStatsInterval = 15 * time.Second
)

type Config struct {
	DiscordToken                 string
	GuildID                      string
	LavalinkHost                 string
	LavalinkPort                 int
	LavalinkPassword             string
	LogLevel                     string
	MetricsEnabled               bool
	MetricsAddr                  string
	MetricsLavalinkStatsInterval time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		DiscordToken:     strings.TrimSpace(os.Getenv("DISCORD_TOKEN")),
		GuildID:          strings.TrimSpace(os.Getenv("GUILD_ID")),
		LavalinkHost:     strings.TrimSpace(os.Getenv("LAVALINK_HOST")),
		LavalinkPassword: strings.TrimSpace(os.Getenv("LAVALINK_PASSWORD")),
		LogLevel:         strings.TrimSpace(os.Getenv("LOG_LEVEL")),
		MetricsEnabled:   true,
		MetricsAddr:      strings.TrimSpace(os.Getenv("METRICS_ADDR")),
	}

	portValue := strings.TrimSpace(os.Getenv("LAVALINK_PORT"))
	metricsEnabledValue := strings.TrimSpace(os.Getenv("METRICS_ENABLED"))
	metricsStatsIntervalValue := strings.TrimSpace(os.Getenv("METRICS_LAVALINK_STATS_INTERVAL"))
	if cfg.LogLevel == "" {
		cfg.LogLevel = defaultLogLevel
	}
	if cfg.MetricsAddr == "" {
		cfg.MetricsAddr = defaultMetricsAddr
	}

	var missing []string
	if cfg.DiscordToken == "" {
		missing = append(missing, "DISCORD_TOKEN")
	}
	if cfg.GuildID == "" {
		missing = append(missing, "GUILD_ID")
	}
	if cfg.LavalinkHost == "" {
		missing = append(missing, "LAVALINK_HOST")
	}
	if cfg.LavalinkPassword == "" {
		missing = append(missing, "LAVALINK_PASSWORD")
	}
	if portValue == "" {
		missing = append(missing, "LAVALINK_PORT")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	port, err := strconv.Atoi(portValue)
	if err != nil {
		return Config{}, fmt.Errorf("invalid LAVALINK_PORT %q: %w", portValue, err)
	}
	if port <= 0 {
		return Config{}, fmt.Errorf("invalid LAVALINK_PORT %q: must be greater than zero", portValue)
	}

	cfg.LavalinkPort = port

	if metricsEnabledValue != "" {
		enabled, err := strconv.ParseBool(metricsEnabledValue)
		if err != nil {
			return Config{}, fmt.Errorf("invalid METRICS_ENABLED %q: %w", metricsEnabledValue, err)
		}
		cfg.MetricsEnabled = enabled
	}

	if metricsStatsIntervalValue == "" {
		cfg.MetricsLavalinkStatsInterval = defaultMetricsLavalinkStatsInterval
	} else {
		interval, err := time.ParseDuration(metricsStatsIntervalValue)
		if err != nil {
			return Config{}, fmt.Errorf("invalid METRICS_LAVALINK_STATS_INTERVAL %q: %w", metricsStatsIntervalValue, err)
		}
		if interval <= 0 {
			return Config{}, fmt.Errorf("invalid METRICS_LAVALINK_STATS_INTERVAL %q: must be greater than zero", metricsStatsIntervalValue)
		}
		cfg.MetricsLavalinkStatsInterval = interval
	}

	return cfg, nil
}

func (c Config) LavalinkAddress() string {
	return fmt.Sprintf("%s:%d", c.LavalinkHost, c.LavalinkPort)
}

func (c Config) LavalinkURL() string {
	return "http://" + c.LavalinkAddress()
}
