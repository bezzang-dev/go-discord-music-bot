package config

import (
	"strings"
	"testing"
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
