package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadSuccess(t *testing.T) {
	setRequiredEnv(t)
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
	if cfg.ShardID != defaultShardID {
		t.Fatalf("unexpected shard ID: %d", cfg.ShardID)
	}
	if cfg.ShardCount != defaultShardCount {
		t.Fatalf("unexpected shard count: %d", cfg.ShardCount)
	}
	if !cfg.DiscordCommandRegistrationEnabled {
		t.Fatal("expected Discord command registration to be enabled by default")
	}
	if !cfg.DiscordCommandCleanupEnabled {
		t.Fatal("expected Discord command cleanup to be enabled by default")
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
	setRequiredEnv(t)
	t.Setenv("LAVALINK_PORT", "invalid")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "LAVALINK_PORT") {
		t.Fatalf("expected port error, got %q", err.Error())
	}
}

func TestLoadAllowsMetricsDisabled(t *testing.T) {
	setRequiredEnv(t)
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
	setRequiredEnv(t)
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
	setRequiredEnv(t)
	t.Setenv("METRICS_LAVALINK_STATS_INTERVAL", "never")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "METRICS_LAVALINK_STATS_INTERVAL") {
		t.Fatalf("expected metrics stats interval error, got %q", err.Error())
	}
}

func TestLoadAllowsShardConfiguration(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SHARD_ID", "1")
	t.Setenv("SHARD_COUNT", "4")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.ShardID != 1 {
		t.Fatalf("unexpected shard ID: %d", cfg.ShardID)
	}
	if cfg.ShardCount != 4 {
		t.Fatalf("unexpected shard count: %d", cfg.ShardCount)
	}
}

func TestLoadRejectsInvalidShardConfiguration(t *testing.T) {
	tests := []struct {
		name       string
		shardID    string
		shardCount string
		wantKey    string
	}{
		{
			name:    "invalid shard ID",
			shardID: "invalid",
			wantKey: "SHARD_ID",
		},
		{
			name:    "negative shard ID",
			shardID: "-1",
			wantKey: "SHARD_ID",
		},
		{
			name:       "invalid shard count",
			shardCount: "invalid",
			wantKey:    "SHARD_COUNT",
		},
		{
			name:       "non-positive shard count",
			shardCount: "0",
			wantKey:    "SHARD_COUNT",
		},
		{
			name:       "shard ID equals shard count",
			shardID:    "2",
			shardCount: "2",
			wantKey:    "SHARD_ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			if tt.shardID != "" {
				t.Setenv("SHARD_ID", tt.shardID)
			}
			if tt.shardCount != "" {
				t.Setenv("SHARD_COUNT", tt.shardCount)
			}

			_, err := Load()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantKey) {
				t.Fatalf("expected %s error, got %q", tt.wantKey, err.Error())
			}
		})
	}
}

func TestLoadAllowsCommandRegistrationDisabled(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("DISCORD_COMMAND_REGISTRATION_ENABLED", "false")
	t.Setenv("DISCORD_COMMAND_CLEANUP_ENABLED", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.DiscordCommandRegistrationEnabled {
		t.Fatal("expected Discord command registration to be disabled")
	}
	if cfg.DiscordCommandCleanupEnabled {
		t.Fatal("expected Discord command cleanup to be disabled")
	}
}

func TestLoadRejectsInvalidCommandRegistrationEnabled(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("DISCORD_COMMAND_REGISTRATION_ENABLED", "maybe")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "DISCORD_COMMAND_REGISTRATION_ENABLED") {
		t.Fatalf("expected command registration error, got %q", err.Error())
	}
}

func TestLoadRejectsInvalidCommandCleanupEnabled(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("DISCORD_COMMAND_CLEANUP_ENABLED", "maybe")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "DISCORD_COMMAND_CLEANUP_ENABLED") {
		t.Fatalf("expected command cleanup error, got %q", err.Error())
	}
}

func setRequiredEnv(t *testing.T) {
	t.Helper()

	t.Setenv("DISCORD_TOKEN", "token")
	t.Setenv("GUILD_ID", "guild")
	t.Setenv("LAVALINK_HOST", "127.0.0.1")
	t.Setenv("LAVALINK_PORT", "2333")
	t.Setenv("LAVALINK_PASSWORD", "password")
}
