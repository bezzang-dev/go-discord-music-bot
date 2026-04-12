# Local Observability Guide

This guide explains how to inspect HNMO Discord Music Bot metrics locally with Prometheus and Grafana.

## Prerequisites
- Run the bot on the host machine.
- Use Docker Compose v2.
- Start Lavalink with the existing local Lavalink guide.

## 1. Configure metrics
Add these optional values to the root `.env` file. The same defaults apply when they are omitted.

```env
METRICS_ENABLED=true
METRICS_ADDR=127.0.0.1:2112
METRICS_LAVALINK_STATS_INTERVAL=15s
```

On Linux, if the Prometheus container cannot reach the host metrics endpoint, run the bot with `METRICS_ADDR=0.0.0.0:2112`.

## 2. Run the bot
Load the environment variables from the repository root and start the bot.

```bash
set -a
source .env
set +a
go run ./cmd/bot
```

Check the startup log for the metrics server.

```text
metrics server is listening on 127.0.0.1:2112
```

Check the metrics endpoint directly.

```bash
curl -sS http://127.0.0.1:2112/metrics
```

A healthy response includes this metric.

```text
hnmo_bot_up 1
```

## 3. Run Prometheus and Grafana
Start the observability stack in a new terminal.

```bash
docker compose -f infra/observability/compose.yml up -d
```

Open Prometheus targets.

```text
http://127.0.0.1:9090/targets
```

The `hnmo-discord-bot` target should be `UP`.

Open Grafana.

```text
http://127.0.0.1:3000
```

The local development credentials are:

```text
admin / admin
```

Open the `HNMO / HNMO Discord Bot` dashboard.

## 4. Check the key metrics
- `hnmo_discord_ready`: `1` after the Discord ready event.
- `hnmo_lavalink_connected`: `1` after the Lavalink WebSocket connects.
- `hnmo_player_active_voice_guilds`: number of guilds where the bot is connected to voice.
- `hnmo_player_playing_guilds`: number of guilds currently playing a track.
- `hnmo_player_queued_tracks`: total queued tracks.
- `hnmo_lavalink_stats_scrape_success`: whether the latest Lavalink `/v4/stats` request succeeded.

The metrics do not expose `guild_id`, user IDs, search queries, or track titles as labels. They use aggregate values to avoid leaking private data and to keep Prometheus label cardinality bounded.

## 5. Stop the stack
Stop only the observability stack.

```bash
docker compose -f infra/observability/compose.yml down
```

Remove the data volumes as well when you want a clean state.

```bash
docker compose -f infra/observability/compose.yml down -v
```
