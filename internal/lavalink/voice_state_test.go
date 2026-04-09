package lavalink

import (
	"context"
	"testing"
	"time"
)

func TestVoiceStateStoreWaitForFullState(t *testing.T) {
	store := NewVoiceStateStore()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	done := make(chan VoiceState, 1)
	go func() {
		state, err := store.WaitForFullState(ctx, "guild-1", "channel-1")
		if err != nil {
			t.Errorf("WaitForFullState returned error: %v", err)
			return
		}
		done <- state
	}()

	store.UpdateVoiceState("guild-1", "session-1", "channel-1")
	store.UpdateVoiceServer("guild-1", "token-1", "endpoint-1")

	select {
	case state := <-done:
		if state.SessionID != "session-1" || state.Token != "token-1" {
			t.Fatalf("unexpected state: %+v", state)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for voice state")
	}
}

func TestVoiceStateStoreRejectsWrongChannel(t *testing.T) {
	store := NewVoiceStateStore()
	store.UpdateVoiceState("guild-1", "session-1", "channel-2")
	store.UpdateVoiceServer("guild-1", "token-1", "endpoint-1")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := store.WaitForFullState(ctx, "guild-1", "channel-1")
	if err == nil {
		t.Fatal("expected error")
	}
}
