# HNMO Discord Music Bot Report

## Document Purpose
This document is an overview for developers who want to understand the current implementation state of the project quickly.

It covers:
- what problem the project solves
- which technologies and structure it uses
- how the bot works at runtime
- how to run and verify it locally
- what is implemented now and which caveats remain

## Project Overview
This project is a Discord music bot written in Go.
The Go process does not handle the Discord voice protocol directly. Actual voice connectivity and playback are delegated to Lavalink.

The currently implemented user commands are:
- `/ping`
- `/play query:<string>`
- `/skip`
- `/stop`
- `/queue`
- `/nowplaying`
- `/leave`

The current goal is to keep the bot stable in a local environment and ready to validate music playback flows in a single guild or a small server.

## Core Tech Stack
### Go application
- Language: Go 1.26.1
- Discord library: `github.com/bwmarrin/discordgo`
- Responsibilities:
  - slash command handling
  - per-guild queue state management
  - Discord interaction responses
  - Lavalink REST and WebSocket control

### Lavalink
- Run method: Docker Compose
- Image: `ghcr.io/lavalink-devs/lavalink:4-alpine`
- Responsibilities:
  - Discord voice connection handling
  - track search and playback
  - playback event delivery

### YouTube source
- Lavalink plugin: `dev.lavalink.youtube:youtube-plugin:1.16.0`
- Search terms are sent to Lavalink with the internal `ytsearch:` prefix.

## Responsibility by Technology
Each technology in the current project has the following role.

### Go
- Main application runtime for the bot
- Handles process startup, configuration loading, the event loop, and per-guild state
- Acts as the control layer between Discord and Lavalink

### `discordgo`
- Handles the Discord Gateway connection
- Receives `READY`, `INTERACTION_CREATE`, `VOICE_STATE_UPDATE`, and `VOICE_SERVER_UPDATE`
- Registers slash commands and sends interaction responses
- Does not stream audio directly. It only relays the state needed for voice connectivity.

### Lavalink
- Joins Discord voice channels and performs actual audio playback
- Handles track search, track loading, player state changes, and playback events
- The Go application controls Lavalink, but Lavalink owns the player engine itself

## Request Flow
### Full runtime flow

```text
User
  -> Discord slash command
  -> discordgo event reception
  -> Go bot handler
  -> internal/player state lookup or update
  -> internal/lavalink REST or WebSocket call
  -> Lavalink
  -> Discord voice channel playback

Reverse event flow:
Lavalink TrackEndEvent
  -> internal/lavalink
  -> Go bot event handler
  -> internal/player next-track selection
  -> Lavalink next-track play request
```

### `/play` request flow

```text
1. User
   -> /play query:<string>

2. Discord
   -> INTERACTION_CREATE

3. Go bot
   -> sends a deferred response
   -> checks the user's current voice channel

4. Go bot
   -> requests a voice join from Discord if needed

5. Discord
   -> VOICE_STATE_UPDATE
   -> VOICE_SERVER_UPDATE

6. Go bot
   -> combines both events into Lavalink voice state
   -> passes that voice state to the Lavalink player

7. Go bot
   -> normalizes the query as a URL or `ytsearch:`
   -> calls Lavalink `loadtracks`

8. Lavalink
   -> searches or loads tracks with the YouTube plugin
   -> returns the first playable track

9. Go bot
   -> updates the current track or queue in `internal/player`
   -> sends a Lavalink play request if the track should start immediately

10. Lavalink
    -> starts actual playback in the Discord voice channel

11. Go bot
    -> edits the interaction response
    -> returns a "Now playing" or "Queued" message
```

### Lavalink YouTube plugin
- Converts YouTube search terms and YouTube URLs into playable track data
- Owns the part where `/play` search input becomes a YouTube search result

### Docker Compose
- Provides a reproducible local Lavalink runtime
- Lets each local machine start Lavalink with the same configuration instead of running a Java process manually

### `.env`
- Stores runtime settings such as the Discord token, test guild ID, and Lavalink host, port, and password
- Keeps code separate from secret values

### `internal/config`
- Reads process environment variables passed from `.env` and validates required values
- Fails early at startup for missing keys or invalid port values

### `internal/lavalink`
- Adapter layer for the Lavalink REST API and WebSocket API
- Implemented features:
  - `/version` check
  - WebSocket session connection
  - track search and load
  - playback start
  - playback stop
  - player deletion
  - playback end event reception
  - Discord voice state delivery in Lavalink format

### `internal/player`
- Domain layer that manages per-guild playback state in memory
- Implemented features:
  - current track storage
  - FIFO queue management
  - next-track selection
  - queue clearing on stop
  - state removal on leave

### `cmd/bot/main.go`
- Real application assembly point
- Wires together `config`, `discordgo`, `lavalink`, and `player` into one bot process
- Command handlers and event handlers are connected here

## Current Directory Structure
The main files and folders are:

```text
cmd/bot/main.go
internal/config/
internal/lavalink/
internal/player/
infra/lavalink/
docs/
```

### `cmd/bot`
- Contains the entry point
- Handles Discord session creation, Lavalink connectivity checks, slash command registration, and event handler wiring

### `internal/config`
- Reads and validates environment variables
- Loads the Discord token, guild ID, and Lavalink host, port, and password

### `internal/lavalink`
- Minimal client layer for Lavalink v4
- Included features:
  - `/version` check
  - Lavalink WebSocket session connection
  - track search and load
  - player updates
  - playback stop
  - player removal
  - helper state storage for Discord voice state delivery

### `internal/player`
- Keeps per-guild playback state in memory
- Stores the current track, queue, and the connected voice channel ID

### `infra/lavalink`
- Contains the Compose and configuration files needed for local Lavalink execution
- `application.yml` includes the Lavalink server and YouTube plugin settings

### `docs`
- Holds operation and overview documents
- The local Lavalink runtime guide is in [lavalink-local.en.md](lavalink-local.en.md)

## Package-by-Package File Guide
The following tables summarize the files you are most likely to read in this repository.

### `cmd/bot` (`package main`)
| File | Role |
| --- | --- |
| [`cmd/bot/main.go`](../cmd/bot/main.go) | The bot entry point and assembly point. It contains configuration loading, Discord session creation, Lavalink connectivity checks, slash command registration, interaction handling, voice state synchronization, and playback control helpers. |

### `internal/config` (`package config`)
| File | Role |
| --- | --- |
| [`internal/config/config.go`](../internal/config/config.go) | Reads and validates runtime configuration from `.env` and process environment variables. It handles missing required keys, invalid port formats, default log level behavior, and helper functions that build Lavalink addresses and URLs. |
| [`internal/config/config_test.go`](../internal/config/config_test.go) | Verifies the success and failure paths of the configuration loader. It tests default log level behavior, missing required variable messages, and invalid port handling. |

### `internal/lavalink` (`package lavalink`)
| File | Role |
| --- | --- |
| [`internal/lavalink/client.go`](../internal/lavalink/client.go) | Core client that wraps the Lavalink REST API and WebSocket API. It is responsible for version checks, session connection, track loading, player updates, play or stop or destroy requests, and event reception with handler wiring. |
| [`internal/lavalink/client_test.go`](../internal/lavalink/client_test.go) | Verifies the Lavalink client's HTTP contract. It tests `/version` response handling, authentication failure messages, empty response handling, and the `loadtracks` track selection logic. |
| [`internal/lavalink/voice_state.go`](../internal/lavalink/voice_state.go) | Stores a complete voice state for Lavalink by combining Discord `VOICE_STATE_UPDATE` and `VOICE_SERVER_UPDATE`. It handles per-guild state storage, waiter wake-up, channel matching checks, and state reset. |
| [`internal/lavalink/voice_state_test.go`](../internal/lavalink/voice_state_test.go) | Verifies synchronization behavior in the voice state store. It tests whether a state is returned only after both events arrive and whether mismatched channel state is rejected. |

### `internal/player` (`package player`)
| File | Role |
| --- | --- |
| [`internal/player/manager.go`](../internal/player/manager.go) | Domain layer that keeps per-guild playback state in memory. It stores the current track, queue, and active voice channel, and provides `enqueue`, `advance`, `stop`, `leave`, and snapshot lookup. |
| [`internal/player/manager_test.go`](../internal/player/manager_test.go) | Verifies player state transition rules. It tests immediate playback for the first track, queueing of later tracks, `advance` behavior, `stop` reset, and state removal on `leave`. |

## Supporting Directories
These are not Go packages, but they matter for local execution and documentation.

### `infra/lavalink`
| File | Role |
| --- | --- |
| [`infra/lavalink/compose.yml`](../infra/lavalink/compose.yml) | Container runtime definition for local Lavalink development. It specifies the image, port binding, password environment variable, configuration file mount, and plugin directory mount. |
| [`infra/lavalink/application.yml`](../infra/lavalink/application.yml) | Lavalink server configuration file. It defines the server port, authentication password, enabled or disabled sources, and YouTube plugin activation and search policy. |

### `docs`
| File | Role |
| --- | --- |
| [`docs/project-overview.md`](project-overview.md) | Korean overview document that explains the overall project structure, runtime flow, run instructions, and current implementation scope. |
| [`docs/project-overview.en.md`](project-overview.en.md) | English overview document for the same project structure, runtime flow, run instructions, and implementation scope. |
| [`docs/lavalink-local.md`](lavalink-local.md) | Korean operations guide for starting Lavalink locally with Docker Compose and checking its state. |
| [`docs/lavalink-local.en.md`](lavalink-local.en.md) | English operations guide for the same local Lavalink startup and verification process. |

## Runtime Structure
The overall shape is:

```text
Discord User
  -> Discord Slash Command
  -> Go Bot
  -> Lavalink
  -> Discord Voice Channel
```

Responsibilities are split as follows.

### Go Bot
- receives Discord interactions
- checks the user's voice channel
- manages per-guild queue state
- sends search, play, and stop commands to Lavalink
- returns results as Discord messages

### Lavalink
- performs Discord voice connections
- searches YouTube and loads tracks
- handles actual playback
- sends track end events

## Major Runtime Flows
### 1. Bot startup flow
1. Read configuration values from `.env`.
2. Check basic connectivity with Lavalink through `/version`.
3. Open the Discord session and wait for the `READY` event.
4. Connect to the Lavalink WebSocket and obtain a `sessionId`.
5. Register slash commands.
6. Wait in the event loop.

### 2. `/play` flow
1. A user calls `/play query:<string>`.
2. The bot sends a deferred interaction response first.
3. It checks the voice channel that the user is currently in.
4. If the bot is not already active in that guild, it requests a voice join from Discord.
5. It receives `VOICE_STATE_UPDATE` and `VOICE_SERVER_UPDATE` and completes the Lavalink voice state.
6. If the input is a search term, it adds `ytsearch:`. If the input is a URL, it passes it directly to Lavalink `loadtracks`.
7. If no track is playing in the current guild, it starts playback immediately.
8. If a track is already playing, it appends the new track to the queue.
9. It edits the interaction response to show the result to the user.

### 3. Automatic next-track flow
1. Lavalink sends a `TrackEndEvent` over WebSocket.
2. The bot takes the next track from the guild queue.
3. If another track exists, it immediately sends a play request to Lavalink.
4. If not, it clears only the current track and becomes idle.

### 4. Control command flows
#### `/skip`
- Skips the current track.
- Plays the next track immediately if one exists.
- Sends a stop request to Lavalink if there is no next track.

#### `/stop`
- Clears the current playback and the queue.
- Sends a stop request to Lavalink as well.

#### `/queue`
- Shows the current track and queued tracks as text.

#### `/nowplaying`
- Shows only the current track.

#### `/leave`
- Removes the guild player state.
- Destroys the Lavalink player.
- Requests leaving the Discord voice channel.

## State Management
The current state model is in-memory only.
That means the queue and current track information disappear when the process restarts.

Per-guild state is managed in [`internal/player/manager.go`](../internal/player/manager.go).

Stored values:
- current connected voice channel ID
- current track
- queue

Strengths of this approach:
- simple and fast
- suitable for initial implementation and local verification

Limits of this approach:
- no persistence
- no state recovery after process restart

The default Gateway runtime model is a single shard.
When shard environment variables are omitted, the bot runs as `SHARD_ID=0` and `SHARD_COUNT=1`.
When multiple shard processes run, Discord guild events are distributed across shards, but the queue and current track state are still stored only in each shard process memory.
Queue recovery after shard restart and state handoff to another shard are not provided yet.

## How To Run
The default run procedure is below.

### 1. Prepare `.env`
Put a `.env` file at the root and fill in the following values:

```env
DISCORD_TOKEN=your-discord-bot-token
GUILD_ID=your-test-guild-id
LAVALINK_HOST=127.0.0.1
LAVALINK_PORT=2333
LAVALINK_PASSWORD=dev-lavalink-pass
LOG_LEVEL=info
SHARD_ID=0
SHARD_COUNT=1
DISCORD_COMMAND_REGISTRATION_ENABLED=true
DISCORD_COMMAND_CLEANUP_ENABLED=true
```

### 2. Start Lavalink

```bash
docker compose -f infra/lavalink/compose.yml up -d
```

Health check:

```bash
docker compose -f infra/lavalink/compose.yml logs --tail=200 lavalink
curl -sS -H "Authorization: ${LAVALINK_PASSWORD}" http://127.0.0.1:2333/version
```

For the detailed local run procedure, see [lavalink-local.en.md](lavalink-local.en.md).

### 3. Run the bot

```bash
set -a
source .env
set +a
go run ./cmd/bot
```

Expected log examples:
- `connected to Lavalink 4.2.2`
- `discord gateway shard configured: shard_id=0 shard_count=1`
- `logged in as ...`
- `Lavalink websocket session ... is ready`
- `bot is running. press Ctrl+C to exit.`

### 4. Run multiple shards
`SHARD_COUNT` must be the same for every process, and `SHARD_ID` must be unique per process from `0` to `SHARD_COUNT - 1`.
Enable slash command registration in only one process, and disable command cleanup for multi-shard operation.

```bash
SHARD_ID=0 SHARD_COUNT=3 DISCORD_COMMAND_REGISTRATION_ENABLED=true DISCORD_COMMAND_CLEANUP_ENABLED=false METRICS_ADDR=127.0.0.1:2112 go run ./cmd/bot
SHARD_ID=1 SHARD_COUNT=3 DISCORD_COMMAND_REGISTRATION_ENABLED=false DISCORD_COMMAND_CLEANUP_ENABLED=false METRICS_ADDR=127.0.0.1:2113 go run ./cmd/bot
SHARD_ID=2 SHARD_COUNT=3 DISCORD_COMMAND_REGISTRATION_ENABLED=false DISCORD_COMMAND_CLEANUP_ENABLED=false METRICS_ADDR=127.0.0.1:2114 go run ./cmd/bot
```

## Manual Verification Procedure
To verify the current implementation, test in a Discord server in this order:

1. Join a test server where the bot is invited.
2. Enter a voice channel first.
3. Run `/play query:<search term or YouTube URL>`.
4. Run `/queue`.
5. Run `/nowplaying`.
6. Run `/skip`.
7. Run `/stop`.
8. Run `/leave`.

Expected results:
- `/play` starts the first track or adds it to the queue.
- `/queue` shows the current track and the queue.
- `/skip` moves to the next track.
- `/stop` clears playback and the queue.
- `/leave` leaves the voice channel.

## Current Implementation Scope
Implemented as of now:
- local Lavalink runtime
- Lavalink startup connectivity verification from the Go app
- Discord slash command registration
- `/ping`
- `/play`
- `/skip`
- `/stop`
- `/queue`
- `/nowplaying`
- `/leave`
- per-guild in-memory queue
- configurable Discord Gateway shard settings
- automatic next-track playback on track end

Not implemented yet:
- `/pause`, `/resume`
- full playlist controls
- persistent storage
- production monitoring
- multi-node Lavalink strategy
- shard startup coordinator and automatic sharding strategy for large public deployment

## Operational Caveats
### Lavalink must start first
The bot verifies Lavalink `/version` and the WebSocket session connection during startup.
If Lavalink is down, the bot does not start.

### `.env` and the Lavalink config password must match
If `LAVALINK_PASSWORD` differs, authentication fails.

### The current queue is in-memory only
If the bot restarts, the current track and queue are lost.

### Register commands from only one process in multi-shard operation
Set `DISCORD_COMMAND_REGISTRATION_ENABLED=true` in only one process.
For multi-shard operation, set `DISCORD_COMMAND_CLEANUP_ENABLED=false` so one shard shutdown does not delete slash commands while other shards are still running.

### Do not use multiple voice channels in the same guild at the same time
The current implementation assumes one active voice channel per guild.
If the bot is already active in another channel, it blocks a new `/play` request.

## Related Documents
- Local Lavalink run guide: [lavalink-local.en.md](lavalink-local.en.md)
- Entry point: [`cmd/bot/main.go`](../cmd/bot/main.go)
- Lavalink client: [`internal/lavalink/client.go`](../internal/lavalink/client.go)
- Guild state management: [`internal/player/manager.go`](../internal/player/manager.go)
