# HNMO Discord Music Bot

This is a Discord music bot built with Go and Lavalink.
Lavalink handles Discord voice connectivity and actual audio playback, while the Go application handles slash commands and per-guild playback state.

## Current Features
- `/ping`
- `/play query:<string>`
- `/skip`
- `/stop`
- `/queue`
- `/nowplaying`
- `/leave`

## Tech Stack
- Go 1.26.1
- `github.com/bwmarrin/discordgo`
- Lavalink v4
- Lavalink YouTube plugin
- Docker Compose

## Request Flow
```text
User
  -> Discord slash command
  -> discordgo event
  -> Go bot handler
  -> internal/player state update
  -> internal/lavalink REST / WebSocket call
  -> Lavalink
  -> Discord voice channel playback

TrackEndEvent
  -> internal/lavalink event handler
  -> Go bot callback
  -> internal/player next track selection
  -> Lavalink play request
```

## Project Structure
```text
cmd/bot/main.go          # Application entry point
internal/config          # Environment loading and validation
internal/lavalink        # Lavalink REST / WebSocket client
internal/player          # Per-guild in-memory queue state
infra/lavalink           # Local Lavalink runtime config
```

## Documents
- English overview: [`docs/project-overview.en.md`](docs/project-overview.en.md)
- English Lavalink guide: [`docs/lavalink-local.en.md`](docs/lavalink-local.en.md)
- Korean overview: [`docs/project-overview.md`](docs/project-overview.md)
- Korean Lavalink guide: [`docs/lavalink-local.md`](docs/lavalink-local.md)

## Prerequisites
Prepare a root `.env` file with the following values:

```env
DISCORD_TOKEN=your-discord-bot-token
GUILD_ID=your-test-guild-id
LAVALINK_HOST=127.0.0.1
LAVALINK_PORT=2333
LAVALINK_PASSWORD=dev-lavalink-pass
LOG_LEVEL=info
```

`.env` is not committed.

## Run
1. Start Lavalink

```bash
docker compose -f infra/lavalink/compose.yml up -d
```

2. Check Lavalink status

```bash
docker compose -f infra/lavalink/compose.yml logs --tail=200 lavalink
curl -sS -H "Authorization: ${LAVALINK_PASSWORD}" http://127.0.0.1:2333/version
```

3. Run the bot

```bash
set -a
source .env
set +a
go run ./cmd/bot
```

Expected startup logs:
- `connected to Lavalink 4.2.2`
- `logged in as ...`
- `Lavalink websocket session ... is ready`
- `bot is running. press Ctrl+C to exit.`

## Manual Verification
In your Discord test server, verify the bot in this order:

1. Join a voice channel
2. `/play query:<search term or YouTube URL>`
3. `/queue`
4. `/nowplaying`
5. `/skip`
6. `/stop`
7. `/leave`

## Notes
- Lavalink must be running before the bot starts.
- `LAVALINK_PASSWORD` must match both `.env` and the Lavalink configuration.
- The current queue state is in-memory only and is lost on bot restart.
