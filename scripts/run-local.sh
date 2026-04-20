#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${ENV_FILE:-"${ROOT_DIR}/.env"}"
START_LAVALINK=true
LAVALINK_WAIT_TIMEOUT_SECONDS="${LAVALINK_WAIT_TIMEOUT_SECONDS:-30}"

usage() {
  cat <<'USAGE'
Usage: scripts/run-local.sh [--skip-lavalink]

Options:
  --skip-lavalink  Do not start the local Lavalink Docker Compose service.
  -h, --help       Show this help message.

Environment:
  ENV_FILE                         Path to the env file. Defaults to .env.
  LAVALINK_WAIT_TIMEOUT_SECONDS    Lavalink readiness timeout. Defaults to 30.
USAGE
}

for arg in "$@"; do
  case "$arg" in
    --skip-lavalink)
      START_LAVALINK=false
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      echo "unknown option: ${arg}" >&2
      usage >&2
      exit 2
      ;;
  esac
done

cd "${ROOT_DIR}"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "env file not found: ${ENV_FILE}" >&2
  echo "create it from the example in README.md, or pass ENV_FILE=/path/to/.env" >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "go command not found" >&2
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "curl command not found" >&2
  exit 1
fi

if [[ "${START_LAVALINK}" == "true" ]] && ! command -v docker >/dev/null 2>&1; then
  echo "docker command not found" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1090
source "${ENV_FILE}"
set +a

missing_vars=()
for var_name in DISCORD_TOKEN GUILD_ID LAVALINK_HOST LAVALINK_PORT LAVALINK_PASSWORD; do
  if [[ -z "${!var_name:-}" ]]; then
    missing_vars+=("${var_name}")
  fi
done

if ((${#missing_vars[@]} > 0)); then
  echo "missing required environment variables: ${missing_vars[*]}" >&2
  exit 1
fi

if [[ "${START_LAVALINK}" == "true" ]]; then
  docker compose -f infra/lavalink/compose.yml up -d
fi

lavalink_url="http://${LAVALINK_HOST}:${LAVALINK_PORT}/version"
echo "waiting for Lavalink at ${lavalink_url}"

deadline=$((SECONDS + LAVALINK_WAIT_TIMEOUT_SECONDS))
until lavalink_version="$(curl -fsS -H "Authorization: ${LAVALINK_PASSWORD}" "${lavalink_url}" 2>/dev/null)"; do
  if ((SECONDS >= deadline)); then
    echo "Lavalink did not become ready within ${LAVALINK_WAIT_TIMEOUT_SECONDS}s" >&2
    echo "check logs with: docker compose -f infra/lavalink/compose.yml logs --tail=200 lavalink" >&2
    exit 1
  fi
  sleep 1
done

echo "Lavalink is ready: ${lavalink_version}"
echo "starting Discord bot"
exec go run ./cmd/bot
