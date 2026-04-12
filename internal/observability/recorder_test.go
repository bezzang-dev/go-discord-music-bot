package observability

import (
	"context"
	"errors"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"

	"github.com/bezzang-dev/go-discord-music-bot/internal/lavalink"
	"github.com/bezzang-dev/go-discord-music-bot/internal/player"
)

type fakeStatsClient struct {
	stats lavalink.Stats
	err   error
}

func (f fakeStatsClient) Stats(context.Context) (lavalink.Stats, error) {
	if f.err != nil {
		return lavalink.Stats{}, f.err
	}
	return f.stats, nil
}

func TestRecorderExposesCoreMetrics(t *testing.T) {
	recorder := NewRecorder(player.NewManager(), nil)

	for _, name := range []string{
		"hnmo_bot_up",
		"hnmo_discord_ready",
		"hnmo_player_known_guilds",
		"hnmo_command_invocations_total",
		"hnmo_lavalink_stats_scrape_success",
	} {
		if gatherMetricFamily(t, recorder, name) == nil {
			t.Fatalf("expected metric %s", name)
		}
	}
}

func TestRecorderReflectsPlayerSummary(t *testing.T) {
	manager := player.NewManager()
	manager.Enqueue("guild", "channel", lavalink.Track{Encoded: "first"})
	manager.Enqueue("guild", "channel", lavalink.Track{Encoded: "second"})
	recorder := NewRecorder(manager, nil)

	assertGaugeValue(t, recorder, "hnmo_player_known_guilds", 1)
	assertGaugeValue(t, recorder, "hnmo_player_active_voice_guilds", 1)
	assertGaugeValue(t, recorder, "hnmo_player_playing_guilds", 1)
	assertGaugeValue(t, recorder, "hnmo_player_queued_tracks", 1)
}

func TestRecordCommandNormalizesLabels(t *testing.T) {
	recorder := NewRecorder(player.NewManager(), nil)

	recorder.RecordCommand(CommandPlay, OutcomeSuccess, 25*time.Millisecond)
	recorder.RecordCommand("not-allowed", "bad", 25*time.Millisecond)

	commandMetric := findMetricByLabels(t, recorder, "hnmo_command_invocations_total", map[string]string{
		"command": CommandPlay,
		"outcome": OutcomeSuccess,
	})
	if commandMetric.GetCounter().GetValue() != 1 {
		t.Fatalf("command counter = %f, want 1", commandMetric.GetCounter().GetValue())
	}

	unknownMetric := findMetricByLabels(t, recorder, "hnmo_command_invocations_total", map[string]string{
		"command": labelUnknown,
		"outcome": labelUnknown,
	})
	if unknownMetric.GetCounter().GetValue() != 1 {
		t.Fatalf("unknown command counter = %f, want 1", unknownMetric.GetCounter().GetValue())
	}
}

func TestRecordLavalinkStatsSuccess(t *testing.T) {
	recorder := NewRecorder(player.NewManager(), nil)
	client := fakeStatsClient{
		stats: lavalink.Stats{
			Players:        2,
			PlayingPlayers: 1,
			Uptime:         30000,
			Memory: lavalink.StatsMemory{
				Free:       1,
				Used:       2,
				Allocated:  3,
				Reservable: 4,
			},
			CPU: lavalink.StatsCPU{
				Cores:        8,
				SystemLoad:   0.5,
				LavalinkLoad: 0.25,
			},
		},
	}

	recorder.recordLavalinkStats(context.Background(), client)

	assertGaugeValue(t, recorder, "hnmo_lavalink_stats_scrape_success", 1)
	assertGaugeValue(t, recorder, "hnmo_lavalink_stats_players", 2)
	assertGaugeValue(t, recorder, "hnmo_lavalink_stats_playing_players", 1)
	assertGaugeValue(t, recorder, "hnmo_lavalink_stats_uptime_seconds", 30)
	assertGaugeValue(t, recorder, "hnmo_lavalink_stats_cpu_cores", 8)
}

func TestRecordLavalinkStatsFailure(t *testing.T) {
	recorder := NewRecorder(player.NewManager(), nil)
	client := fakeStatsClient{err: errors.New("stats failed")}

	recorder.recordLavalinkStats(context.Background(), client)

	assertGaugeValue(t, recorder, "hnmo_lavalink_stats_scrape_success", 0)
}

func gatherMetricFamily(t *testing.T, recorder *Recorder, name string) *dto.MetricFamily {
	t.Helper()

	families, err := recorder.registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	for _, family := range families {
		if family.GetName() == name {
			return family
		}
	}
	return nil
}

func assertGaugeValue(t *testing.T, recorder *Recorder, name string, want float64) {
	t.Helper()

	family := gatherMetricFamily(t, recorder, name)
	if family == nil || len(family.GetMetric()) == 0 {
		t.Fatalf("expected gauge metric %s", name)
	}

	got := family.GetMetric()[0].GetGauge().GetValue()
	if got != want {
		t.Fatalf("%s = %f, want %f", name, got, want)
	}
}

func findMetricByLabels(t *testing.T, recorder *Recorder, name string, labels map[string]string) *dto.Metric {
	t.Helper()

	family := gatherMetricFamily(t, recorder, name)
	if family == nil {
		t.Fatalf("expected metric %s", name)
	}

	for _, metric := range family.GetMetric() {
		if metricHasLabels(metric, labels) {
			return metric
		}
	}

	t.Fatalf("expected metric %s with labels %+v", name, labels)
	return nil
}

func metricHasLabels(metric *dto.Metric, labels map[string]string) bool {
	for name, want := range labels {
		found := false
		for _, pair := range metric.GetLabel() {
			if pair.GetName() == name && pair.GetValue() == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
