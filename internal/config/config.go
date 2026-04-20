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
	defaultShardID                      = 0
	defaultShardCount                   = 1
)

type Config struct {
	DiscordToken                      string
	GuildID                           string
	LavalinkHost                      string
	LavalinkPort                      int
	LavalinkPassword                  string
	LogLevel                          string
	MetricsEnabled                    bool
	MetricsAddr                       string
	MetricsLavalinkStatsInterval      time.Duration
	ShardID                           int
	ShardCount                        int
	DiscordCommandRegistrationEnabled bool
	DiscordCommandCleanupEnabled      bool
}

func Load() (Config, error) {
	cfg := Config{
		DiscordToken:                      strings.TrimSpace(os.Getenv("DISCORD_TOKEN")),
		GuildID:                           strings.TrimSpace(os.Getenv("GUILD_ID")),
		LavalinkHost:                      strings.TrimSpace(os.Getenv("LAVALINK_HOST")),
		LavalinkPassword:                  strings.TrimSpace(os.Getenv("LAVALINK_PASSWORD")),
		LogLevel:                          strings.TrimSpace(os.Getenv("LOG_LEVEL")),
		MetricsEnabled:                    true,
		MetricsAddr:                       strings.TrimSpace(os.Getenv("METRICS_ADDR")),
		ShardID:                           defaultShardID,
		ShardCount:                        defaultShardCount,
		DiscordCommandRegistrationEnabled: true,
		DiscordCommandCleanupEnabled:      true,
	}

	portValue := strings.TrimSpace(os.Getenv("LAVALINK_PORT"))
	metricsEnabledValue := strings.TrimSpace(os.Getenv("METRICS_ENABLED"))
	metricsStatsIntervalValue := strings.TrimSpace(os.Getenv("METRICS_LAVALINK_STATS_INTERVAL"))
	shardIDValue := strings.TrimSpace(os.Getenv("SHARD_ID"))
	shardCountValue := strings.TrimSpace(os.Getenv("SHARD_COUNT"))
	commandRegistrationEnabledValue := strings.TrimSpace(os.Getenv("DISCORD_COMMAND_REGISTRATION_ENABLED"))
	commandCleanupEnabledValue := strings.TrimSpace(os.Getenv("DISCORD_COMMAND_CLEANUP_ENABLED"))
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

	if shardIDValue != "" {
		shardID, err := strconv.Atoi(shardIDValue)
		if err != nil {
			return Config{}, fmt.Errorf("invalid SHARD_ID %q: %w", shardIDValue, err)
		}
		if shardID < 0 {
			return Config{}, fmt.Errorf("invalid SHARD_ID %q: must be greater than or equal to zero", shardIDValue)
		}
		cfg.ShardID = shardID
	}

	if shardCountValue != "" {
		shardCount, err := strconv.Atoi(shardCountValue)
		if err != nil {
			return Config{}, fmt.Errorf("invalid SHARD_COUNT %q: %w", shardCountValue, err)
		}
		if shardCount <= 0 {
			return Config{}, fmt.Errorf("invalid SHARD_COUNT %q: must be greater than zero", shardCountValue)
		}
		cfg.ShardCount = shardCount
	}

	if cfg.ShardID >= cfg.ShardCount {
		return Config{}, fmt.Errorf("invalid shard configuration: SHARD_ID %d must be less than SHARD_COUNT %d", cfg.ShardID, cfg.ShardCount)
	}

	if commandRegistrationEnabledValue != "" {
		enabled, err := strconv.ParseBool(commandRegistrationEnabledValue)
		if err != nil {
			return Config{}, fmt.Errorf("invalid DISCORD_COMMAND_REGISTRATION_ENABLED %q: %w", commandRegistrationEnabledValue, err)
		}
		cfg.DiscordCommandRegistrationEnabled = enabled
	}

	if commandCleanupEnabledValue != "" {
		enabled, err := strconv.ParseBool(commandCleanupEnabledValue)
		if err != nil {
			return Config{}, fmt.Errorf("invalid DISCORD_COMMAND_CLEANUP_ENABLED %q: %w", commandCleanupEnabledValue, err)
		}
		cfg.DiscordCommandCleanupEnabled = enabled
	}

	return cfg, nil
}

func (c Config) LavalinkAddress() string {
	return fmt.Sprintf("%s:%d", c.LavalinkHost, c.LavalinkPort)
}

func (c Config) LavalinkURL() string {
	return "http://" + c.LavalinkAddress()
}
