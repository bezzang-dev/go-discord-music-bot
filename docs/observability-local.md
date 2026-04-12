# 로컬 관측성 실행 가이드

이 문서는 로컬에서 HNMO Discord Music Bot의 Prometheus metrics와 Grafana dashboard를 확인하는 절차를 설명한다.

## 전제 조건
- 봇은 호스트 머신에서 실행한다.
- Docker Compose v2를 사용할 수 있어야 한다.
- Lavalink는 기존 가이드대로 실행되어 있어야 한다.

## 1. metrics 환경 변수 설정
루트 `.env`에 아래 값을 추가한다. 모두 선택 값이며, 생략하면 같은 기본값이 적용된다.

```env
METRICS_ENABLED=true
METRICS_ADDR=127.0.0.1:2112
METRICS_LAVALINK_STATS_INTERVAL=15s
```

Linux에서 Prometheus 컨테이너가 호스트의 metrics endpoint에 접근하지 못하면 `METRICS_ADDR=0.0.0.0:2112`로 바꿔서 봇을 실행한다.

## 2. 봇 실행
루트에서 환경 변수를 로드한 뒤 봇을 실행한다.

```bash
set -a
source .env
set +a
go run ./cmd/bot
```

시작 로그에서 아래 내용을 확인한다.

```text
metrics server is listening on 127.0.0.1:2112
```

metrics endpoint를 직접 확인한다.

```bash
curl -sS http://127.0.0.1:2112/metrics
```

정상이라면 아래 metric이 포함된다.

```text
hnmo_bot_up 1
```

## 3. Prometheus와 Grafana 실행
새 터미널에서 관측성 스택을 실행한다.

```bash
docker compose -f infra/observability/compose.yml up -d
```

Prometheus target 상태를 확인한다.

```text
http://127.0.0.1:9090/targets
```

`hnmo-discord-bot` target이 `UP`이면 Prometheus가 봇 metrics를 수집 중이다.

Grafana를 연다.

```text
http://127.0.0.1:3000
```

기본 계정은 로컬 개발용이다.

```text
admin / admin
```

Grafana에서 `HNMO / HNMO Discord Bot` dashboard를 연다.

## 4. 확인할 값
- `hnmo_discord_ready`: Discord ready 이벤트를 받으면 `1`.
- `hnmo_lavalink_connected`: Lavalink WebSocket 연결 후 `1`.
- `hnmo_player_active_voice_guilds`: 봇이 음성 채널에 연결된 서버 수.
- `hnmo_player_playing_guilds`: 현재 재생 중인 서버 수.
- `hnmo_player_queued_tracks`: 전체 대기열 곡 수.
- `hnmo_lavalink_stats_scrape_success`: Lavalink `/v4/stats` 조회 성공 여부.

서버별 `guild_id`, 사용자 ID, 검색어, 트랙 제목은 label로 노출하지 않는다. 개인정보 노출과 Prometheus label cardinality 증가를 피하기 위해 집계 값만 제공한다.

## 5. 종료
관측성 스택만 종료한다.

```bash
docker compose -f infra/observability/compose.yml down
```

데이터 볼륨까지 지우려면 다음 명령을 사용한다.

```bash
docker compose -f infra/observability/compose.yml down -v
```
