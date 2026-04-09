# Lavalink 로컬 실행 가이드

## 목적
이 문서는 이 저장소에서 Lavalink를 로컬로 실행하고 상태를 확인하는 최소 절차를 정리한다.

## 사전 조건
- Docker Desktop 또는 Docker Engine + Docker Compose v2
- Java 17 이상
- 저장소 루트에 `.env` 파일 존재

## 1. `.env` 준비
루트에서 `.env.example`을 기준으로 `.env`를 채운다.
`docker compose -f infra/lavalink/compose.yml ...`를 저장소 루트에서 실행하면 루트 `.env` 값이 치환에 사용된다.

필수 키:
- `DISCORD_TOKEN`
- `GUILD_ID`
- `LAVALINK_HOST`
- `LAVALINK_PORT`
- `LAVALINK_PASSWORD`

권장 로컬 값:

```env
LAVALINK_HOST=127.0.0.1
LAVALINK_PORT=2333
LAVALINK_PASSWORD=dev-lavalink-pass
LOG_LEVEL=info
```

## 2. Lavalink 기동
저장소 루트에서 아래 명령을 실행한다.

```bash
docker compose -f infra/lavalink/compose.yml up -d
```

정상 기동 여부를 확인한다.

```bash
docker compose -f infra/lavalink/compose.yml logs --tail=200 lavalink
curl -sS -H "Authorization: ${LAVALINK_PASSWORD}" http://127.0.0.1:2333/version
```

현재 확인한 로컬 이미지에서는 `/version`도 `Authorization` 헤더가 있어야 정상 응답한다.
버전 문자열이 반환되면 Lavalink 프로세스는 떠 있는 상태다.

## 3. Lavalink 중지

```bash
docker compose -f infra/lavalink/compose.yml down
```

플러그인 캐시는 `infra/lavalink/plugins/`에 남는다.

## 4. Bot 실행 전 환경 변수 반영
현재 Go 애플리케이션은 셸 환경 변수를 직접 읽는다. `.env` 파일을 자동으로 읽지는 않는다.
또한 시작 시점에 Lavalink `/version` 확인을 수행하므로, Lavalink가 먼저 떠 있어야 봇이 실행된다.

```bash
set -a
source .env
set +a
go run ./cmd/bot
```

정상이라면 봇 시작 로그에 Lavalink 대상 주소와 연결된 버전이 출력된다. 비밀번호는 출력되지 않는다.

## 자주 나는 문제
### 2333 포트가 이미 사용 중인 경우
`bind: address already in use`가 나오면 로컬에서 2333 포트를 점유한 프로세스를 먼저 정리한다.

확인 예시:

```bash
lsof -nP -iTCP:2333 -sTCP:LISTEN
```

### Authorization mismatch
Go 앱의 `LAVALINK_PASSWORD`와 `infra/lavalink/application.yml`에서 실제 적용된 비밀번호가 다르면 시작 시점에 인증 오류가 난다.

우선 아래 두 값을 맞춘다.
- 루트 `.env`의 `LAVALINK_PASSWORD`
- 컨테이너에 전달되는 `LAVALINK_PASSWORD`

변경 후에는 컨테이너를 다시 띄운다.

```bash
docker compose -f infra/lavalink/compose.yml down
docker compose -f infra/lavalink/compose.yml up -d
```

### 플러그인 다운로드 실패
초기 기동 시 `youtube-plugin` jar 다운로드가 실패하면 네트워크 또는 원격 저장소 접근 문제일 가능성이 높다.

확인 순서:
- `docker compose -f infra/lavalink/compose.yml logs --tail=200 lavalink`
- `infra/lavalink/plugins/`에 플러그인 파일이 생성됐는지 확인
- 잠시 후 컨테이너 재기동

## 다음 단계
다음 구현 묶음은 길드별 큐와 `/skip`, `/stop`, `/queue`, `/nowplaying`, `/leave` 같은 제어 커맨드를 붙이는 단계다.
