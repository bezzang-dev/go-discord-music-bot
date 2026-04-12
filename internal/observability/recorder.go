package observability

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/bezzang-dev/go-discord-music-bot/internal/lavalink"
	"github.com/bezzang-dev/go-discord-music-bot/internal/player"
)

const (
	CommandPing       = "ping"
	CommandPlay       = "play"
	CommandSkip       = "skip"
	CommandStop       = "stop"
	CommandQueue      = "queue"
	CommandNowPlaying = "nowplaying"
	CommandLeave      = "leave"

	OutcomeSuccess         = "success"
	OutcomeNoop            = "noop"
	OutcomeUserError       = "user_error"
	OutcomeDependencyError = "dependency_error"

	labelUnknown = "unknown"
	labelNone    = "none"
)

type lavalinkStatsClient interface {
	Stats(context.Context) (lavalink.Stats, error)
}

type Recorder struct {
	registry *prometheus.Registry
	players  *player.Manager
	lavalink *lavalink.Client

	discordReady      prometheus.Gauge
	discordReadyGuild prometheus.Gauge
	lavalinkConnected prometheus.Gauge

	commandInvocations *prometheus.CounterVec
	commandDuration    *prometheus.HistogramVec
	lavalinkEvents     *prometheus.CounterVec

	lavalinkStatsPlayers        prometheus.Gauge
	lavalinkStatsPlayingPlayers prometheus.Gauge
	lavalinkStatsUptimeSeconds  prometheus.Gauge
	lavalinkStatsMemoryBytes    *prometheus.GaugeVec
	lavalinkStatsCPUCores       prometheus.Gauge
	lavalinkStatsCPULoadRatio   *prometheus.GaugeVec
	lavalinkStatsScrapeSuccess  prometheus.Gauge
	lavalinkStatsScrapeDuration prometheus.Histogram
}

func NewRecorder(players *player.Manager, lavalinkClient *lavalink.Client) *Recorder {
	recorder := &Recorder{
		registry: prometheus.NewRegistry(),
		players:  players,
		lavalink: lavalinkClient,
		discordReady: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "hnmo_discord_ready",
			Help: "Whether Discord sent the ready event.",
		}),
		discordReadyGuild: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "hnmo_discord_ready_guilds",
			Help: "Number of guilds included in the Discord ready event.",
		}),
		lavalinkConnected: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "hnmo_lavalink_connected",
			Help: "Whether the Lavalink websocket session is connected.",
		}),
		commandInvocations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "hnmo_command_invocations_total",
			Help: "Total Discord slash command invocations.",
		}, []string{"command", "outcome"}),
		commandDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "hnmo_command_duration_seconds",
			Help:    "Discord slash command handling duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"command", "outcome"}),
		lavalinkEvents: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "hnmo_lavalink_events_total",
			Help: "Total Lavalink websocket events.",
		}, []string{"event_type", "reason"}),
		lavalinkStatsPlayers: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "hnmo_lavalink_stats_players",
			Help: "Number of Lavalink players reported by /v4/stats.",
		}),
		lavalinkStatsPlayingPlayers: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "hnmo_lavalink_stats_playing_players",
			Help: "Number of actively playing Lavalink players reported by /v4/stats.",
		}),
		lavalinkStatsUptimeSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "hnmo_lavalink_stats_uptime_seconds",
			Help: "Lavalink uptime reported by /v4/stats in seconds.",
		}),
		lavalinkStatsMemoryBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "hnmo_lavalink_stats_memory_bytes",
			Help: "Lavalink memory statistics reported by /v4/stats.",
		}, []string{"kind"}),
		lavalinkStatsCPUCores: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "hnmo_lavalink_stats_cpu_cores",
			Help: "Lavalink CPU core count reported by /v4/stats.",
		}),
		lavalinkStatsCPULoadRatio: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "hnmo_lavalink_stats_cpu_load_ratio",
			Help: "Lavalink CPU load ratio reported by /v4/stats.",
		}, []string{"kind"}),
		lavalinkStatsScrapeSuccess: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "hnmo_lavalink_stats_scrape_success",
			Help: "Whether the most recent Lavalink stats scrape succeeded.",
		}),
		lavalinkStatsScrapeDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "hnmo_lavalink_stats_scrape_duration_seconds",
			Help:    "Duration of Lavalink stats scrape requests in seconds.",
			Buckets: prometheus.DefBuckets,
		}),
	}

	recorder.register()
	recorder.initializeLabelValues()
	return recorder
}

func (r *Recorder) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(r.registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func (r *Recorder) RecordCommand(command string, outcome string, duration time.Duration) {
	command = normalizeCommand(command)
	outcome = normalizeOutcome(outcome)
	r.commandInvocations.WithLabelValues(command, outcome).Inc()
	r.commandDuration.WithLabelValues(command, outcome).Observe(duration.Seconds())
}

func (r *Recorder) RecordLavalinkEvent(eventType string, reason string) {
	if eventType == "" {
		eventType = labelUnknown
	}
	if reason == "" {
		reason = labelNone
	}
	r.lavalinkEvents.WithLabelValues(eventType, reason).Inc()
}

func (r *Recorder) SetDiscordReady(guildCount int) {
	r.discordReady.Set(1)
	r.discordReadyGuild.Set(float64(guildCount))
}

func (r *Recorder) SetLavalinkConnected(connected bool) {
	if connected {
		r.lavalinkConnected.Set(1)
		return
	}
	r.lavalinkConnected.Set(0)
}

func (r *Recorder) StartLavalinkStatsPolling(ctx context.Context, client *lavalink.Client, interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	go func() {
		if client != nil {
			r.SetLavalinkConnected(client.Connected())
		}
		r.recordLavalinkStats(ctx, client)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if client != nil {
					r.SetLavalinkConnected(client.Connected())
				}
				r.recordLavalinkStats(ctx, client)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (r *Recorder) register() {
	r.registry.MustRegister(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "hnmo_bot_up",
			Help: "Whether the bot process is exposing metrics.",
		}, func() float64 {
			return 1
		}),
		r.discordReady,
		r.discordReadyGuild,
		r.lavalinkConnected,
		r.playerSummaryGauge("hnmo_player_known_guilds", "Number of guild playback states known by the in-memory player manager.", func(summary player.Summary) int {
			return summary.KnownGuilds
		}),
		r.playerSummaryGauge("hnmo_player_active_voice_guilds", "Number of guilds with an active voice channel.", func(summary player.Summary) int {
			return summary.ActiveVoiceGuilds
		}),
		r.playerSummaryGauge("hnmo_player_playing_guilds", "Number of guilds currently playing a track.", func(summary player.Summary) int {
			return summary.PlayingGuilds
		}),
		r.playerSummaryGauge("hnmo_player_queued_tracks", "Total queued tracks across all guilds.", func(summary player.Summary) int {
			return summary.QueuedTracks
		}),
		r.commandInvocations,
		r.commandDuration,
		r.lavalinkEvents,
		r.lavalinkStatsPlayers,
		r.lavalinkStatsPlayingPlayers,
		r.lavalinkStatsUptimeSeconds,
		r.lavalinkStatsMemoryBytes,
		r.lavalinkStatsCPUCores,
		r.lavalinkStatsCPULoadRatio,
		r.lavalinkStatsScrapeSuccess,
		r.lavalinkStatsScrapeDuration,
	)
}

func (r *Recorder) playerSummaryGauge(name string, help string, value func(player.Summary) int) prometheus.Collector {
	return prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: name,
		Help: help,
	}, func() float64 {
		if r.players == nil {
			return 0
		}
		return float64(value(r.players.Summary()))
	})
}

func (r *Recorder) recordLavalinkStats(ctx context.Context, client lavalinkStatsClient) {
	if client == nil {
		r.lavalinkStatsScrapeSuccess.Set(0)
		return
	}

	startedAt := time.Now()
	statsCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	stats, err := client.Stats(statsCtx)
	r.lavalinkStatsScrapeDuration.Observe(time.Since(startedAt).Seconds())
	if err != nil {
		r.lavalinkStatsScrapeSuccess.Set(0)
		return
	}

	r.lavalinkStatsScrapeSuccess.Set(1)
	r.lavalinkStatsPlayers.Set(float64(stats.Players))
	r.lavalinkStatsPlayingPlayers.Set(float64(stats.PlayingPlayers))
	r.lavalinkStatsUptimeSeconds.Set(float64(stats.Uptime) / 1000)
	r.lavalinkStatsMemoryBytes.WithLabelValues("free").Set(float64(stats.Memory.Free))
	r.lavalinkStatsMemoryBytes.WithLabelValues("used").Set(float64(stats.Memory.Used))
	r.lavalinkStatsMemoryBytes.WithLabelValues("allocated").Set(float64(stats.Memory.Allocated))
	r.lavalinkStatsMemoryBytes.WithLabelValues("reservable").Set(float64(stats.Memory.Reservable))
	r.lavalinkStatsCPUCores.Set(float64(stats.CPU.Cores))
	r.lavalinkStatsCPULoadRatio.WithLabelValues("system").Set(stats.CPU.SystemLoad)
	r.lavalinkStatsCPULoadRatio.WithLabelValues("lavalink").Set(stats.CPU.LavalinkLoad)
}

func (r *Recorder) initializeLabelValues() {
	for _, command := range []string{
		CommandPing,
		CommandPlay,
		CommandSkip,
		CommandStop,
		CommandQueue,
		CommandNowPlaying,
		CommandLeave,
		labelUnknown,
	} {
		for _, outcome := range []string{
			OutcomeSuccess,
			OutcomeNoop,
			OutcomeUserError,
			OutcomeDependencyError,
			labelUnknown,
		} {
			r.commandInvocations.WithLabelValues(command, outcome)
			r.commandDuration.WithLabelValues(command, outcome)
		}
	}
	for _, kind := range []string{"free", "used", "allocated", "reservable"} {
		r.lavalinkStatsMemoryBytes.WithLabelValues(kind)
	}
	for _, kind := range []string{"system", "lavalink"} {
		r.lavalinkStatsCPULoadRatio.WithLabelValues(kind)
	}
}

func normalizeCommand(command string) string {
	switch command {
	case CommandPing, CommandPlay, CommandSkip, CommandStop, CommandQueue, CommandNowPlaying, CommandLeave:
		return command
	default:
		return labelUnknown
	}
}

func normalizeOutcome(outcome string) string {
	switch outcome {
	case OutcomeSuccess, OutcomeNoop, OutcomeUserError, OutcomeDependencyError:
		return outcome
	default:
		return labelUnknown
	}
}
