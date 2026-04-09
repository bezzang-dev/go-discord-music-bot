package player

import (
	"sync"

	"github.com/bezzang-dev/go-discord-music-bot/internal/lavalink"
)

type Snapshot struct {
	VoiceChannelID string
	Current        *lavalink.Track
	Queue          []lavalink.Track
}

type EnqueueResult struct {
	Started       bool
	QueuePosition int
	Track         lavalink.Track
}

type Manager struct {
	mu      sync.Mutex
	players map[string]*guildPlayer
}

type guildPlayer struct {
	voiceChannelID string
	current        *lavalink.Track
	queue          []lavalink.Track
}

func NewManager() *Manager {
	return &Manager{
		players: make(map[string]*guildPlayer),
	}
}

func (m *Manager) Enqueue(guildID, voiceChannelID string, track lavalink.Track) EnqueueResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	player := m.getOrCreateLocked(guildID)
	player.voiceChannelID = voiceChannelID

	if player.current == nil {
		trackCopy := track
		player.current = &trackCopy
		return EnqueueResult{
			Started: true,
			Track:   track,
		}
	}

	player.queue = append(player.queue, track)
	return EnqueueResult{
		Started:       false,
		QueuePosition: len(player.queue),
		Track:         track,
	}
}

func (m *Manager) Snapshot(guildID string) Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	player, ok := m.players[guildID]
	if !ok {
		return Snapshot{}
	}

	snapshot := Snapshot{
		VoiceChannelID: player.voiceChannelID,
		Queue:          append([]lavalink.Track(nil), player.queue...),
	}
	if player.current != nil {
		current := *player.current
		snapshot.Current = &current
	}

	return snapshot
}

func (m *Manager) Advance(guildID string) (*lavalink.Track, string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	player, ok := m.players[guildID]
	if !ok || player.current == nil {
		return nil, "", false
	}

	if len(player.queue) == 0 {
		player.current = nil
		return nil, player.voiceChannelID, true
	}

	next := player.queue[0]
	player.queue = player.queue[1:]
	nextCopy := next
	player.current = &nextCopy

	return &next, player.voiceChannelID, true
}

func (m *Manager) Stop(guildID string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	player, ok := m.players[guildID]
	if !ok {
		return "", false
	}

	player.current = nil
	player.queue = nil
	return player.voiceChannelID, true
}

func (m *Manager) Leave(guildID string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	player, ok := m.players[guildID]
	if !ok {
		return "", false
	}

	voiceChannelID := player.voiceChannelID
	delete(m.players, guildID)
	return voiceChannelID, true
}

func (m *Manager) SetVoiceChannel(guildID, voiceChannelID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	player, ok := m.players[guildID]
	if !ok {
		if voiceChannelID == "" {
			return
		}
		player = m.getOrCreateLocked(guildID)
	}
	player.voiceChannelID = voiceChannelID
}

func (m *Manager) ClearCurrent(guildID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	player, ok := m.players[guildID]
	if !ok {
		return
	}

	player.current = nil
}

func (m *Manager) getOrCreateLocked(guildID string) *guildPlayer {
	player, ok := m.players[guildID]
	if ok {
		return player
	}

	player = &guildPlayer{}
	m.players[guildID] = player
	return player
}
