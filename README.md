# HNMO Discord Music Bot

Go와 Lavalink로 구성한 Discord 음악 봇입니다.
Discord 음성 연결과 실제 재생은 Lavalink가 담당하고, Go 애플리케이션은 슬래시 커맨드 처리와 길드별 재생 상태 관리를 담당합니다.

## 현재 구현 기능
- `/ping`
- `/play query:<string>`
- `/skip`
- `/stop`
- `/queue`
- `/nowplaying`
- `/leave`

## 기술 구성
- Go 1.26.1
- `github.com/bwmarrin/discordgo`
- Lavalink v4
- Lavalink YouTube 플러그인
- Docker Compose

## 프로젝트 구조
```text
cmd/bot/main.go          # 애플리케이션 진입점
internal/config          # 환경 변수 로드 및 검증
internal/lavalink        # Lavalink REST / WebSocket 클라이언트
internal/player          # 길드별 메모리 큐 상태 관리
infra/lavalink           # 로컬 Lavalink 실행 설정
```

## 실행 전 준비
루트 `.env` 파일에 아래 값을 준비합니다.

```env
DISCORD_TOKEN=your-discord-bot-token
GUILD_ID=your-test-guild-id
LAVALINK_HOST=127.0.0.1
LAVALINK_PORT=2333
LAVALINK_PASSWORD=dev-lavalink-pass
LOG_LEVEL=info
```

`.env`는 커밋 대상이 아닙니다.

## 실행 방법
1. Lavalink 실행

```bash
docker compose -f infra/lavalink/compose.yml up -d
```

2. Lavalink 상태 확인

```bash
docker compose -f infra/lavalink/compose.yml logs --tail=200 lavalink
curl -sS -H "Authorization: ${LAVALINK_PASSWORD}" http://127.0.0.1:2333/version
```

3. 봇 실행

```bash
set -a
source .env
set +a
go run ./cmd/bot
```

정상 시작 로그 예시:
- `connected to Lavalink 4.2.2`
- `logged in as ...`
- `Lavalink websocket session ... is ready`
- `bot is running. press Ctrl+C to exit.`

## 확인 순서
Discord 서버에서 아래 순서로 동작을 확인하면 됩니다.

1. 음성 채널 입장
2. `/play query:<검색어 또는 유튜브 URL>`
3. `/queue`
4. `/nowplaying`
5. `/skip`
6. `/stop`
7. `/leave`

## 주의사항
- Lavalink가 먼저 떠 있어야 봇이 시작됩니다.
- `LAVALINK_PASSWORD`는 `.env`와 Lavalink 설정에서 같아야 합니다.
- 현재 큐 상태는 메모리 기반이라 봇 재시작 시 사라집니다.
