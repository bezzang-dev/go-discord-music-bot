package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadSuccess(t *testing.T) {
	t.Setenv("DISCORD_TOKEN", "token")
	t.Setenv("GUILD_ID", "guild")
	t.Setenv("LAVALINK_HOST", "127.0.0.1")
	t.Setenv("LAVALINK_PORT", "2333")
	t.Setenv("LAVALINK_PASSWORD", "password")
	t.Setenv("LOG_LEVEL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.LogLevel != defaultLogLevel {
		t.Fatalf("expected default log level %q, got %q", defaultLogLevel, cfg.LogLevel)
	}
	if cfg.LavalinkAddress() != "127.0.0.1:2333" {
		t.Fatalf("unexpected lavalink address: %s", cfg.LavalinkAddress())
	}
	if cfg.LavalinkURL() != "http://127.0.0.1:2333" {
		t.Fatalf("unexpected lavalink url: %s", cfg.LavalinkURL())
	}
	if !cfg.MetricsEnabled {
		t.Fatal("expected metrics to be enabled by default")
	}
	if cfg.MetricsAddr != defaultMetricsAddr {
		t.Fatalf("unexpected metrics address: %s", cfg.MetricsAddr)
	}
	if cfg.MetricsLavalinkStatsInterval != defaultMetricsLavalinkStatsInterval {
		t.Fatalf("unexpected metrics stats interval: %s", cfg.MetricsLavalinkStatsInterval)
	}
}

func TestLoadMissingRequiredKeys(t *testing.T) {
	t.Setenv("DISCORD_TOKEN", "")
	t.Setenv("GUILD_ID", "")
	t.Setenv("LAVALINK_HOST", "")
	t.Setenv("LAVALINK_PORT", "")
	t.Setenv("LAVALINK_PASSWORD", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}

	message := err.Error()
	for _, key := range []string{"DISCORD_TOKEN", "GUILD_ID", "LAVALINK_HOST", "LAVALINK_PORT", "LAVALINK_PASSWORD"} {
		if !strings.Contains(message, key) {
			t.Fatalf("expected missing key %s in error %q", key, message)
		}
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("DISCORD_TOKEN", "token")
	t.Setenv("GUILD_ID", "guild")
	t.Setenv("LAVALINK_HOST", "127.0.0.1")
	t.Setenv("LAVALINK_PORT", "invalid")
	t.Setenv("LAVALINK_PASSWORD", "password")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "LAVALINK_PORT") {
		t.Fatalf("expected port error, got %q", err.Error())
	}
}

func TestLoadAllowsMetricsDisabled(t *testing.T) {
	t.Setenv("DISCORD_TOKEN", "token")
	t.Setenv("GUILD_ID", "guild")
	t.Setenv("LAVALINK_HOST", "127.0.0.1")
	t.Setenv("LAVALINK_PORT", "2333")
	t.Setenv("LAVALINK_PASSWORD", "password")
	t.Setenv("METRICS_ENABLED", "false")
	t.Setenv("METRICS_ADDR", "127.0.0.1:2113")
	t.Setenv("METRICS_LAVALINK_STATS_INTERVAL", "30s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.MetricsEnabled {
		t.Fatal("expected metrics to be disabled")
	}
	if cfg.MetricsAddr != "127.0.0.1:2113" {
		t.Fatalf("unexpected metrics address: %s", cfg.MetricsAddr)
	}
	if cfg.MetricsLavalinkStatsInterval != 30*time.Second {
		t.Fatalf("unexpected metrics stats interval: %s", cfg.MetricsLavalinkStatsInterval)
	}
}

func TestLoadRejectsInvalidMetricsEnabled(t *testing.T) {
	t.Setenv("DISCORD_TOKEN", "token")
	t.Setenv("GUILD_ID", "guild")
	t.Setenv("LAVALINK_HOST", "127.0.0.1")
	t.Setenv("LAVALINK_PORT", "2333")
	t.Setenv("LAVALINK_PASSWORD", "password")
	t.Setenv("METRICS_ENABLED", "maybe")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "METRICS_ENABLED") {
		t.Fatalf("expected metrics enabled error, got %q", err.Error())
	}
}

func TestLoadRejectsInvalidMetricsStatsInterval(t *testing.T) {
	t.Setenv("DISCORD_TOKEN", "token")
	t.Setenv("GUILD_ID", "guild")
	t.Setenv("LAVALINK_HOST", "127.0.0.1")
	t.Setenv("LAVALINK_PORT", "2333")
	t.Setenv("LAVALINK_PASSWORD", "password")
	t.Setenv("METRICS_LAVALINK_STATS_INTERVAL", "never")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "METRICS_LAVALINK_STATS_INTERVAL") {
		t.Fatalf("expected metrics stats interval error, got %q", err.Error())
	}
}
