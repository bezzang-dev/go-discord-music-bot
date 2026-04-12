package main

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestVoiceStateTouchesChannel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		update    *discordgo.VoiceStateUpdate
		channelID string
		want      bool
	}{
		{
			name:      "current channel matches",
			update:    &discordgo.VoiceStateUpdate{VoiceState: &discordgo.VoiceState{ChannelID: "voice-1"}},
			channelID: "voice-1",
			want:      true,
		},
		{
			name: "before update channel matches",
			update: &discordgo.VoiceStateUpdate{
				VoiceState:   &discordgo.VoiceState{ChannelID: "voice-2"},
				BeforeUpdate: &discordgo.VoiceState{ChannelID: "voice-1"},
			},
			channelID: "voice-1",
			want:      true,
		},
		{
			name:      "unrelated channel",
			update:    &discordgo.VoiceStateUpdate{VoiceState: &discordgo.VoiceState{ChannelID: "voice-2"}},
			channelID: "voice-1",
			want:      false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := voiceStateTouchesChannel(tt.update, tt.channelID); got != tt.want {
				t.Fatalf("voiceStateTouchesChannel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCountHumanUsersInChannel(t *testing.T) {
	t.Parallel()

	voiceStates := []*discordgo.VoiceState{
		{UserID: "bot-self", ChannelID: "voice-1"},
		{UserID: "user-1", ChannelID: "voice-1"},
		{UserID: "bot-2", ChannelID: "voice-1"},
		{UserID: "user-2", ChannelID: "voice-2"},
	}

	got := countHumanUsersInChannel(voiceStates, "voice-1", "bot-self", func(voiceState *discordgo.VoiceState) bool {
		return voiceState.UserID == "bot-2"
	})

	if got != 1 {
		t.Fatalf("countHumanUsersInChannel() = %d, want 1", got)
	}
}
