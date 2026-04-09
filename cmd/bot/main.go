package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/bezzang-dev/go-discord-music-bot/internal/config"
	"github.com/bezzang-dev/go-discord-music-bot/internal/lavalink"
	"github.com/bezzang-dev/go-discord-music-bot/internal/player"
)

const (
	playCommandName       = "play"
	skipCommandName       = "skip"
	stopCommandName       = "stop"
	queueCommandName      = "queue"
	nowPlayingCommandName = "nowplaying"
	leaveCommandName      = "leave"
)

type bot struct {
	cfg          config.Config
	discord      *discordgo.Session
	lavalink     *lavalink.Client
	voiceState   *lavalink.VoiceStateStore
	players      *player.Manager
	readyOnce    sync.Once
	discordReady chan struct{}
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	lavalinkClient := lavalink.NewClient(cfg.LavalinkURL(), cfg.LavalinkPassword, &http.Client{
		Timeout: 5 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Startup assumes Lavalink is available, so fail fast before opening Discord.
	version, err := lavalinkClient.Version(ctx)
	if err != nil {
		log.Fatalf("failed to connect to Lavalink at %s: %v", cfg.LavalinkAddress(), err)
	}
	log.Printf("connected to Lavalink %s", version)

	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		log.Fatalf("failed to create discord session: %v", err)
	}

	app := &bot{
		cfg:          cfg,
		discord:      session,
		lavalink:     lavalinkClient,
		voiceState:   lavalink.NewVoiceStateStore(),
		players:      player.NewManager(),
		discordReady: make(chan struct{}),
	}
	app.lavalink.SetEventHandler(app.onLavalinkEvent)

	session.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildVoiceStates
	session.AddHandler(app.onReady)
	session.AddHandler(app.onVoiceStateUpdate)
	session.AddHandler(app.onVoiceServerUpdate)
	session.AddHandler(app.onInteractionCreate)

	if err := session.Open(); err != nil {
		log.Fatalf("failed to open discord session: %v", err)
	}
	defer session.Close()

	if err := app.waitForDiscordReady(); err != nil {
		log.Fatal(err)
	}

	wsCtx, wsCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer wsCancel()

	if err := app.lavalink.Connect(wsCtx, session.State.User.ID); err != nil {
		log.Fatalf("failed to establish Lavalink websocket session: %v", err)
	}
	defer func() {
		if err := app.lavalink.Close(); err != nil {
			log.Printf("failed to close lavalink websocket: %v", err)
		}
	}()
	log.Printf("Lavalink websocket session %s is ready", app.lavalink.SessionID())

	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "ping",
			Description: "Check whether the bot is alive",
		},
		{
			Name:        playCommandName,
			Description: "Play a YouTube search result or URL through Lavalink",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "query",
					Description: "YouTube URL or search query",
					Required:    true,
				},
			},
		},
		{
			Name:        skipCommandName,
			Description: "Skip the current track",
		},
		{
			Name:        stopCommandName,
			Description: "Stop playback and clear the queue",
		},
		{
			Name:        queueCommandName,
			Description: "Show the current queue",
		},
		{
			Name:        nowPlayingCommandName,
			Description: "Show the currently playing track",
		},
		{
			Name:        leaveCommandName,
			Description: "Disconnect from the active voice channel",
		},
	}

	// Guild-scoped commands keep local development fast and avoid global propagation delays.
	createdCommands := make([]*discordgo.ApplicationCommand, 0, len(commands))
	for _, command := range commands {
		createdCommand, err := session.ApplicationCommandCreate(
			session.State.User.ID,
			cfg.GuildID,
			command,
		)
		if err != nil {
			log.Fatalf("failed to register slash command %s: %v", command.Name, err)
		}
		createdCommands = append(createdCommands, createdCommand)
	}

	defer func() {
		for _, command := range createdCommands {
			err := session.ApplicationCommandDelete(session.State.User.ID, cfg.GuildID, command.ID)
			if err != nil {
				log.Printf("failed to delete slash command %s: %v", command.Name, err)
			}
		}
	}()

	log.Println("bot is running. press Ctrl+C to exit.")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down bot")
}

func (b *bot) onReady(s *discordgo.Session, r *discordgo.Ready) {
	log.Printf("logged in as %s#%s", r.User.Username, r.User.Discriminator)
	b.readyOnce.Do(func() {
		close(b.discordReady)
	})
}

func (b *bot) onVoiceStateUpdate(s *discordgo.Session, update *discordgo.VoiceStateUpdate) {
	if s.State == nil || s.State.User == nil {
		return
	}
	// Lavalink only needs the bot's own Discord voice session details.
	if update.UserID != s.State.User.ID {
		return
	}
	if update.ChannelID == "" {
		b.voiceState.Clear(update.GuildID)
		b.players.SetVoiceChannel(update.GuildID, "")
		return
	}
	b.voiceState.UpdateVoiceState(update.GuildID, update.SessionID, update.ChannelID)
	b.players.SetVoiceChannel(update.GuildID, update.ChannelID)
}

func (b *bot) onVoiceServerUpdate(s *discordgo.Session, update *discordgo.VoiceServerUpdate) {
	b.voiceState.UpdateVoiceServer(update.GuildID, update.Token, update.Endpoint)
}

func (b *bot) onLavalinkEvent(event lavalink.Event) {
	if event.Op != "event" {
		return
	}

	switch event.Type {
	case "TrackEndEvent":
		// Only terminal endings should pull from the queue. User-driven skips/stops are handled elsewhere.
		reason := strings.ToLower(event.Reason)
		if reason != "finished" && reason != "loadfailed" {
			return
		}

		next, _, ok := b.players.Advance(event.GuildID)
		if !ok || next == nil {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := b.lavalink.PlayTrack(ctx, event.GuildID, *next); err != nil {
			log.Printf("failed to auto-play next track for guild %s: %v", event.GuildID, err)
		}
	case "WebSocketClosedEvent":
		log.Printf("lavalink websocket closed for guild %s", event.GuildID)
	}
}

func (b *bot) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	switch i.ApplicationCommandData().Name {
	case "ping":
		b.respondImmediate(i, "pong")
	case playCommandName:
		b.handlePlay(i)
	case skipCommandName:
		b.handleSkip(i)
	case stopCommandName:
		b.handleStop(i)
	case queueCommandName:
		b.handleQueue(i)
	case nowPlayingCommandName:
		b.handleNowPlaying(i)
	case leaveCommandName:
		b.handleLeave(i)
	}
}

// handlePlay joins the caller's voice channel if needed, resolves a track, and either starts playback or queues it.
func (b *bot) handlePlay(i *discordgo.InteractionCreate) {
	if !b.deferInteraction(i) {
		return
	}

	query := strings.TrimSpace(i.ApplicationCommandData().Options[0].StringValue())
	if query == "" {
		b.editInteraction(i, "query is required")
		return
	}

	channelID, err := findUserVoiceChannel(b.discord, i.GuildID, i.Member.User.ID)
	if err != nil {
		b.editInteraction(i, err.Error())
		return
	}

	snapshot := b.players.Snapshot(i.GuildID)
	if snapshot.VoiceChannelID != "" && snapshot.VoiceChannelID != channelID {
		b.editInteraction(i, "bot is already active in another voice channel")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := b.ensureVoiceConnection(ctx, i.GuildID, channelID); err != nil {
		b.editInteraction(i, err.Error())
		log.Printf("failed to prepare voice connection for guild %s: %v", i.GuildID, err)
		return
	}

	track, err := b.lavalink.LoadTrack(ctx, normalizeTrackIdentifier(query))
	if err != nil {
		b.editInteraction(i, fmt.Sprintf("failed to load track: %v", err))
		log.Printf("failed to load track for guild %s query %q: %v", i.GuildID, query, err)
		return
	}

	result := b.players.Enqueue(i.GuildID, channelID, track)
	if result.Started {
		if err := b.lavalink.PlayTrack(ctx, i.GuildID, track); err != nil {
			b.players.ClearCurrent(i.GuildID)
			b.editInteraction(i, "failed to start playback in Lavalink")
			log.Printf("failed to play track for guild %s: %v", i.GuildID, err)
			return
		}

		message := fmt.Sprintf("Now playing: **%s** by **%s**", track.Info.Title, track.Info.Author)
		b.editInteraction(i, message)
		return
	}

	message := fmt.Sprintf("Queued #%d: **%s** by **%s**", result.QueuePosition, track.Info.Title, track.Info.Author)
	b.editInteraction(i, message)
}

// handleSkip advances the guild queue and keeps Lavalink in sync with the next playback state.
func (b *bot) handleSkip(i *discordgo.InteractionCreate) {
	if !b.deferInteraction(i) {
		return
	}

	snapshot := b.players.Snapshot(i.GuildID)
	if snapshot.Current == nil {
		b.editInteraction(i, "there is no track to skip")
		return
	}

	if err := b.requireUserInActiveChannel(i, snapshot.VoiceChannelID); err != nil {
		b.editInteraction(i, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	next, _, _ := b.players.Advance(i.GuildID)
	if next == nil {
		if err := b.lavalink.StopTrack(ctx, i.GuildID); err != nil {
			b.editInteraction(i, "failed to stop playback in Lavalink")
			log.Printf("failed to stop playback for guild %s: %v", i.GuildID, err)
			return
		}
		b.editInteraction(i, "Skipped the current track. Queue is now empty.")
		return
	}

	if err := b.lavalink.PlayTrack(ctx, i.GuildID, *next); err != nil {
		b.editInteraction(i, "failed to play the next track")
		log.Printf("failed to skip to next track for guild %s: %v", i.GuildID, err)
		return
	}

	message := fmt.Sprintf("Skipped. Now playing: **%s** by **%s**", next.Info.Title, next.Info.Author)
	b.editInteraction(i, message)
}

// handleStop clears the in-memory queue first, then stops the Lavalink player for the guild.
func (b *bot) handleStop(i *discordgo.InteractionCreate) {
	if !b.deferInteraction(i) {
		return
	}

	snapshot := b.players.Snapshot(i.GuildID)
	if snapshot.Current == nil && len(snapshot.Queue) == 0 {
		b.editInteraction(i, "there is no active queue to stop")
		return
	}

	if err := b.requireUserInActiveChannel(i, snapshot.VoiceChannelID); err != nil {
		b.editInteraction(i, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, ok := b.players.Stop(i.GuildID); !ok {
		b.editInteraction(i, "there is no active queue to stop")
		return
	}

	if err := b.lavalink.StopTrack(ctx, i.GuildID); err != nil {
		b.editInteraction(i, "failed to stop playback in Lavalink")
		log.Printf("failed to stop playback for guild %s: %v", i.GuildID, err)
		return
	}

	b.editInteraction(i, fmt.Sprintf("Stopped playback and cleared %d queued track(s).", len(snapshot.Queue)))
}

func (b *bot) handleQueue(i *discordgo.InteractionCreate) {
	snapshot := b.players.Snapshot(i.GuildID)
	if snapshot.Current == nil && len(snapshot.Queue) == 0 {
		b.respondImmediate(i, "queue is empty")
		return
	}

	var lines []string
	if snapshot.Current != nil {
		lines = append(lines, fmt.Sprintf("Now playing: **%s** by **%s** (%s)", snapshot.Current.Info.Title, snapshot.Current.Info.Author, formatDuration(snapshot.Current.Info.Length)))
	}
	if len(snapshot.Queue) == 0 {
		lines = append(lines, "Up next: nothing queued")
	} else {
		lines = append(lines, "Up next:")
		for index, track := range snapshot.Queue {
			lines = append(lines, fmt.Sprintf("%d. **%s** by **%s** (%s)", index+1, track.Info.Title, track.Info.Author, formatDuration(track.Info.Length)))
			if index >= 9 {
				if remaining := len(snapshot.Queue) - index - 1; remaining > 0 {
					lines = append(lines, fmt.Sprintf("...and %d more track(s)", remaining))
				}
				break
			}
		}
	}

	b.respondImmediate(i, strings.Join(lines, "\n"))
}

func (b *bot) handleNowPlaying(i *discordgo.InteractionCreate) {
	snapshot := b.players.Snapshot(i.GuildID)
	if snapshot.Current == nil {
		b.respondImmediate(i, "nothing is playing right now")
		return
	}

	message := fmt.Sprintf("Now playing: **%s** by **%s** (%s)", snapshot.Current.Info.Title, snapshot.Current.Info.Author, formatDuration(snapshot.Current.Info.Length))
	b.respondImmediate(i, message)
}

// handleLeave tears down both the local guild state and the remote Lavalink player before disconnecting from Discord voice.
func (b *bot) handleLeave(i *discordgo.InteractionCreate) {
	if !b.deferInteraction(i) {
		return
	}

	snapshot := b.players.Snapshot(i.GuildID)
	if snapshot.VoiceChannelID == "" {
		b.editInteraction(i, "bot is not connected to a voice channel")
		return
	}

	if err := b.requireUserInActiveChannel(i, snapshot.VoiceChannelID); err != nil {
		b.editInteraction(i, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	b.players.Leave(i.GuildID)
	b.voiceState.Clear(i.GuildID)

	if err := b.lavalink.DestroyPlayer(ctx, i.GuildID); err != nil {
		b.editInteraction(i, "failed to destroy the Lavalink player")
		log.Printf("failed to destroy player for guild %s: %v", i.GuildID, err)
		return
	}

	if err := b.discord.ChannelVoiceJoinManual(i.GuildID, "", false, true); err != nil {
		b.editInteraction(i, "failed to disconnect from the voice channel")
		log.Printf("failed to send voice disconnect for guild %s: %v", i.GuildID, err)
		return
	}

	b.editInteraction(i, "Disconnected from the voice channel and cleared the queue.")
}

func (b *bot) deferInteraction(i *discordgo.InteractionCreate) bool {
	err := b.discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		log.Printf("failed to defer interaction %s: %v", i.ApplicationCommandData().Name, err)
		return false
	}
	return true
}

func (b *bot) respondImmediate(i *discordgo.InteractionCreate, content string) {
	err := b.discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	})
	if err != nil {
		log.Printf("failed to respond to interaction: %v", err)
	}
}

func (b *bot) editInteraction(i *discordgo.InteractionCreate, content string) {
	_, err := b.discord.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
	})
	if err != nil {
		log.Printf("failed to edit interaction response: %v", err)
	}
}

// ensureVoiceConnection makes sure Discord has acknowledged the voice join and forwards the complete state to Lavalink.
func (b *bot) ensureVoiceConnection(ctx context.Context, guildID, channelID string) error {
	snapshot := b.players.Snapshot(guildID)
	if snapshot.VoiceChannelID == "" {
		if err := b.discord.ChannelVoiceJoinManual(guildID, channelID, false, true); err != nil {
			return fmt.Errorf("failed to ask Discord to join the voice channel: %w", err)
		}
	}

	// Discord sends voice state and voice server updates separately; Lavalink needs the merged result.
	voiceState, err := b.voiceState.WaitForFullState(ctx, guildID, channelID)
	if err != nil {
		return fmt.Errorf("timed out while waiting for Lavalink voice state: %w", err)
	}

	if err := b.lavalink.UpdateVoiceState(ctx, guildID, voiceState); err != nil {
		return fmt.Errorf("failed to forward voice state to Lavalink: %w", err)
	}

	b.players.SetVoiceChannel(guildID, channelID)
	return nil
}

func (b *bot) requireUserInActiveChannel(i *discordgo.InteractionCreate, activeChannelID string) error {
	channelID, err := findUserVoiceChannel(b.discord, i.GuildID, i.Member.User.ID)
	if err != nil {
		return err
	}
	if activeChannelID != "" && channelID != activeChannelID {
		return fmt.Errorf("you must be in the active voice channel first")
	}
	return nil
}

func (b *bot) waitForDiscordReady() error {
	select {
	case <-b.discordReady:
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timed out waiting for Discord ready event")
	}
}

func findUserVoiceChannel(s *discordgo.Session, guildID, userID string) (string, error) {
	voiceState, err := s.State.VoiceState(guildID, userID)
	if err != nil || voiceState == nil || voiceState.ChannelID == "" {
		return "", fmt.Errorf("you must join a voice channel first")
	}

	return voiceState.ChannelID, nil
}

func normalizeTrackIdentifier(query string) string {
	if parsed, err := url.Parse(query); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		return query
	}

	// Non-URL input is treated as a YouTube search handled by the Lavalink plugin.
	return "ytsearch:" + query
}

func formatDuration(milliseconds int64) string {
	if milliseconds <= 0 {
		return "live"
	}

	totalSeconds := milliseconds / 1000
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}
