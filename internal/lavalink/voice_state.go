package lavalink

import (
	"context"
	"fmt"
	"sync"
)

type VoiceStateStore struct {
	mu      sync.Mutex
	states  map[string]VoiceState
	waiters map[string][]chan struct{}
}

func NewVoiceStateStore() *VoiceStateStore {
	return &VoiceStateStore{
		states:  make(map[string]VoiceState),
		waiters: make(map[string][]chan struct{}),
	}
}

func (s *VoiceStateStore) UpdateVoiceState(guildID, sessionID, channelID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.states[guildID]
	state.SessionID = sessionID
	state.ChannelID = channelID
	s.states[guildID] = state
	s.notifyLocked(guildID)
}

func (s *VoiceStateStore) UpdateVoiceServer(guildID, token, endpoint string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.states[guildID]
	state.Token = token
	state.Endpoint = endpoint
	s.states[guildID] = state
	s.notifyLocked(guildID)
}

func (s *VoiceStateStore) Clear(guildID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.states, guildID)
	s.notifyLocked(guildID)
}

func (s *VoiceStateStore) WaitForFullState(ctx context.Context, guildID, channelID string) (VoiceState, error) {
	s.mu.Lock()
	if state, ok := s.states[guildID]; ok && state.completeFor(channelID) {
		s.mu.Unlock()
		return state, nil
	}

	waiter := make(chan struct{}, 1)
	s.waiters[guildID] = append(s.waiters[guildID], waiter)
	s.mu.Unlock()

	defer s.removeWaiter(guildID, waiter)

	for {
		select {
		case <-ctx.Done():
			return VoiceState{}, fmt.Errorf("wait for voice state for guild %s: %w", guildID, ctx.Err())
		case <-waiter:
			s.mu.Lock()
			state, ok := s.states[guildID]
			s.mu.Unlock()
			if ok && state.completeFor(channelID) {
				return state, nil
			}
		}
	}
}

func (s *VoiceStateStore) removeWaiter(guildID string, target chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	waiters := s.waiters[guildID]
	for i, waiter := range waiters {
		if waiter == target {
			s.waiters[guildID] = append(waiters[:i], waiters[i+1:]...)
			break
		}
	}
	if len(s.waiters[guildID]) == 0 {
		delete(s.waiters, guildID)
	}
}

func (s *VoiceStateStore) notifyLocked(guildID string) {
	for _, waiter := range s.waiters[guildID] {
		select {
		case waiter <- struct{}{}:
		default:
		}
	}
}

func (s VoiceState) completeFor(channelID string) bool {
	if s.Token == "" || s.Endpoint == "" || s.SessionID == "" || s.ChannelID == "" {
		return false
	}
	if channelID != "" && s.ChannelID != channelID {
		return false
	}
	return true
}
