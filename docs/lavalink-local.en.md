# Lavalink Local Run Guide

## Purpose
This document lists the minimum steps to run Lavalink locally in this repository and verify that it is healthy.

## Prerequisites
- Docker Desktop or Docker Engine with Docker Compose v2
- Java 17 or later
- A `.env` file at the repository root

## 1. Prepare `.env`
Fill in the root `.env` file based on `.env.example`.
If you run `docker compose -f infra/lavalink/compose.yml ...` from the repository root, Docker Compose uses the root `.env` file for variable substitution.

Required keys:
- `DISCORD_TOKEN`
- `GUILD_ID`
- `LAVALINK_HOST`
- `LAVALINK_PORT`
- `LAVALINK_PASSWORD`

Recommended local values:

```env
LAVALINK_HOST=127.0.0.1
LAVALINK_PORT=2333
LAVALINK_PASSWORD=dev-lavalink-pass
LOG_LEVEL=info
```

## 2. Start Lavalink
Run the following command from the repository root:

```bash
docker compose -f infra/lavalink/compose.yml up -d
```

Check that the service started correctly:

```bash
docker compose -f infra/lavalink/compose.yml logs --tail=200 lavalink
curl -sS -H "Authorization: ${LAVALINK_PASSWORD}" http://127.0.0.1:2333/version
```

On the local image currently used by this repository, `/version` also requires the `Authorization` header.
If the command returns a version string, the Lavalink process is up.

## 3. Stop Lavalink

```bash
docker compose -f infra/lavalink/compose.yml down
```

The plugin cache remains under `infra/lavalink/plugins/`.

## 4. Export Environment Variables Before Running the Bot
The current Go application reads shell environment variables directly. It does not load `.env` automatically.
It also checks Lavalink `/version` during startup, so Lavalink must already be running before you start the bot.

```bash
set -a
source .env
set +a
go run ./cmd/bot
```

If startup succeeds, the bot logs show the Lavalink target address and the connected version. The password is not printed.

## Common Issues
### Port 2333 Already in Use
If you see `bind: address already in use`, stop the process that is already listening on port 2333.

Example check:

```bash
lsof -nP -iTCP:2333 -sTCP:LISTEN
```

### Authorization Mismatch
If the Go app `LAVALINK_PASSWORD` does not match the password actually applied in `infra/lavalink/application.yml`, startup fails with an authentication error.

Check these values first:
- `LAVALINK_PASSWORD` in the root `.env`
- `LAVALINK_PASSWORD` passed into the container

After changing the password, restart the container:

```bash
docker compose -f infra/lavalink/compose.yml down
docker compose -f infra/lavalink/compose.yml up -d
```

### Plugin Download Failure
If the `youtube-plugin` jar download fails during the first startup, the cause is usually a network or remote repository access problem.

Check in this order:
- `docker compose -f infra/lavalink/compose.yml logs --tail=200 lavalink`
- Whether the plugin file exists under `infra/lavalink/plugins/`
- Restart the container after a short wait

## Next Step
The next implementation bundle is to expand guild playback controls such as `/skip`, `/stop`, `/queue`, `/nowplaying`, and `/leave`.
