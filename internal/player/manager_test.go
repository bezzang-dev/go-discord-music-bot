package player

import (
	"testing"

	"github.com/bezzang-dev/go-discord-music-bot/internal/lavalink"
)

func TestEnqueueStartsThenQueues(t *testing.T) {
	manager := NewManager()

	first := lavalink.Track{Encoded: "first", Info: lavalink.TrackInfo{Title: "first"}}
	second := lavalink.Track{Encoded: "second", Info: lavalink.TrackInfo{Title: "second"}}

	started := manager.Enqueue("guild", "channel", first)
	if !started.Started {
		t.Fatal("expected first track to start immediately")
	}

	queued := manager.Enqueue("guild", "channel", second)
	if queued.Started {
		t.Fatal("expected second track to be queued")
	}
	if queued.QueuePosition != 1 {
		t.Fatalf("expected queue position 1, got %d", queued.QueuePosition)
	}

	snapshot := manager.Snapshot("guild")
	if snapshot.Current == nil || snapshot.Current.Encoded != "first" {
		t.Fatalf("unexpected current track: %+v", snapshot.Current)
	}
	if len(snapshot.Queue) != 1 || snapshot.Queue[0].Encoded != "second" {
		t.Fatalf("unexpected queue: %+v", snapshot.Queue)
	}
}

func TestAdvanceMovesQueueForward(t *testing.T) {
	manager := NewManager()
	manager.Enqueue("guild", "channel", lavalink.Track{Encoded: "first"})
	manager.Enqueue("guild", "channel", lavalink.Track{Encoded: "second"})

	next, channelID, ok := manager.Advance("guild")
	if !ok {
		t.Fatal("expected active player")
	}
	if channelID != "channel" {
		t.Fatalf("unexpected channel ID %q", channelID)
	}
	if next == nil || next.Encoded != "second" {
		t.Fatalf("unexpected next track %+v", next)
	}

	snapshot := manager.Snapshot("guild")
	if snapshot.Current == nil || snapshot.Current.Encoded != "second" {
		t.Fatalf("unexpected current track: %+v", snapshot.Current)
	}
	if len(snapshot.Queue) != 0 {
		t.Fatalf("expected empty queue, got %+v", snapshot.Queue)
	}
}

func TestStopClearsCurrentAndQueue(t *testing.T) {
	manager := NewManager()
	manager.Enqueue("guild", "channel", lavalink.Track{Encoded: "first"})
	manager.Enqueue("guild", "channel", lavalink.Track{Encoded: "second"})

	channelID, ok := manager.Stop("guild")
	if !ok {
		t.Fatal("expected player to exist")
	}
	if channelID != "channel" {
		t.Fatalf("unexpected channel ID %q", channelID)
	}

	snapshot := manager.Snapshot("guild")
	if snapshot.Current != nil {
		t.Fatalf("expected no current track, got %+v", snapshot.Current)
	}
	if len(snapshot.Queue) != 0 {
		t.Fatalf("expected empty queue, got %+v", snapshot.Queue)
	}
}

func TestLeaveRemovesPlayer(t *testing.T) {
	manager := NewManager()
	manager.Enqueue("guild", "channel", lavalink.Track{Encoded: "first"})

	channelID, ok := manager.Leave("guild")
	if !ok {
		t.Fatal("expected player to exist")
	}
	if channelID != "channel" {
		t.Fatalf("unexpected channel ID %q", channelID)
	}

	snapshot := manager.Snapshot("guild")
	if snapshot.Current != nil || len(snapshot.Queue) != 0 || snapshot.VoiceChannelID != "" {
		t.Fatalf("expected empty snapshot, got %+v", snapshot)
	}
}

func TestSummaryEmptyManager(t *testing.T) {
	manager := NewManager()

	summary := manager.Summary()
	if summary != (Summary{}) {
		t.Fatalf("expected empty summary, got %+v", summary)
	}
}

func TestSummaryAggregatesPlaybackState(t *testing.T) {
	manager := NewManager()
	manager.Enqueue("guild-1", "channel-1", lavalink.Track{Encoded: "first"})
	manager.Enqueue("guild-1", "channel-1", lavalink.Track{Encoded: "second"})
	manager.SetVoiceChannel("guild-2", "channel-2")

	summary := manager.Summary()
	if summary.KnownGuilds != 2 {
		t.Fatalf("KnownGuilds = %d, want 2", summary.KnownGuilds)
	}
	if summary.ActiveVoiceGuilds != 2 {
		t.Fatalf("ActiveVoiceGuilds = %d, want 2", summary.ActiveVoiceGuilds)
	}
	if summary.PlayingGuilds != 1 {
		t.Fatalf("PlayingGuilds = %d, want 1", summary.PlayingGuilds)
	}
	if summary.QueuedTracks != 1 {
		t.Fatalf("QueuedTracks = %d, want 1", summary.QueuedTracks)
	}
}

func TestSummaryReflectsLeave(t *testing.T) {
	manager := NewManager()
	manager.Enqueue("guild", "channel", lavalink.Track{Encoded: "first"})
	manager.Enqueue("guild", "channel", lavalink.Track{Encoded: "second"})

	manager.Leave("guild")

	summary := manager.Summary()
	if summary != (Summary{}) {
		t.Fatalf("expected empty summary after leave, got %+v", summary)
	}
}
