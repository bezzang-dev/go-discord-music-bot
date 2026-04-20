# HNMO Discord Music Bot 보고서

## 문서 목적
이 문서는 현재 프로젝트의 구현 상태를 빠르게 파악하려는 개발자를 위한 설명 문서다.

다음 내용을 다룬다.
- 프로젝트가 어떤 문제를 해결하는지
- 어떤 기술과 구조로 구성되어 있는지
- 봇이 어떤 흐름으로 동작하는지
- 로컬에서 어떻게 실행하고 검증하는지
- 현재 구현 범위와 남은 주의사항이 무엇인지

## 프로젝트 개요
이 프로젝트는 Go로 작성한 Discord 음악 봇이다.
Discord 음성 프로토콜을 Go 프로세스가 직접 처리하지 않고, 실제 음성 연결과 재생은 Lavalink에 위임한다.

현재 구현된 사용자 커맨드는 다음과 같다.
- `/ping`
- `/play query:<string>`
- `/skip`
- `/stop`
- `/queue`
- `/nowplaying`
- `/leave`

현재 목표는 로컬 환경에서 안정적으로 실행되고, 단일 길드 또는 소규모 서버에서 음악 재생 흐름을 검증할 수 있는 상태를 유지하는 것이다.

## 핵심 기술 스택
### Go 애플리케이션
- 언어: Go 1.26.1
- Discord 라이브러리: `github.com/bwmarrin/discordgo`
- 역할:
  - 슬래시 커맨드 처리
  - 길드별 큐 상태 관리
  - Discord interaction 응답 처리
  - Lavalink REST / WebSocket 제어

### Lavalink
- 실행 방식: Docker Compose
- 이미지: `ghcr.io/lavalink-devs/lavalink:4-alpine`
- 역할:
  - Discord 음성 연결 처리
  - 트랙 검색 및 재생
  - 재생 관련 이벤트 전송

### YouTube 소스
- Lavalink 플러그인: `dev.lavalink.youtube:youtube-plugin:1.16.0`
- 검색어는 내부적으로 `ytsearch:` 접두어를 사용해 Lavalink에 전달한다.

## 기술별 담당 기능
현재 프로젝트에서 각 기술은 아래 역할을 담당한다.

### Go
- 봇의 메인 애플리케이션 런타임이다.
- 프로세스 시작, 설정 로드, 이벤트 루프, 길드별 상태 관리를 담당한다.
- Discord와 Lavalink를 연결하는 제어 계층 역할을 맡는다.

### `discordgo`
- Discord Gateway 연결을 담당한다.
- `READY`, `INTERACTION_CREATE`, `VOICE_STATE_UPDATE`, `VOICE_SERVER_UPDATE` 이벤트를 수신한다.
- 슬래시 커맨드 등록과 interaction 응답 전송을 처리한다.
- 직접 오디오를 송출하지는 않고, 음성 연결에 필요한 상태 이벤트만 중계한다.

### Lavalink
- Discord 음성 채널 접속과 실제 오디오 재생을 담당한다.
- 트랙 검색, 트랙 로드, 플레이어 상태 변경, 재생 이벤트 전송을 처리한다.
- Go 애플리케이션은 Lavalink를 제어만 하고 실제 플레이어 엔진은 Lavalink가 맡는다.

## 요청 흐름도
### 전체 런타임 흐름도

```text
사용자
  -> Discord 슬래시 커맨드
  -> discordgo 이벤트 수신
  -> Go 봇 핸들러
  -> internal/player 상태 조회/갱신
  -> internal/lavalink REST/WebSocket 호출
  -> Lavalink
  -> Discord 음성 채널 재생

반대 방향 이벤트:
Lavalink TrackEndEvent
  -> internal/lavalink
  -> Go 봇 이벤트 핸들러
  -> internal/player 다음 곡 선택
  -> Lavalink 다음 곡 재생 요청
```

### `/play` 요청 흐름도

```text
1. 사용자
   -> /play query:<string>

2. Discord
   -> INTERACTION_CREATE

3. Go 봇
   -> deferred response 전송
   -> 사용자의 현재 voice channel 확인

4. Go 봇
   -> 필요 시 Discord에 voice join 요청

5. Discord
   -> VOICE_STATE_UPDATE
   -> VOICE_SERVER_UPDATE

6. Go 봇
   -> 두 이벤트를 합쳐 Lavalink용 voice state 구성
   -> Lavalink player에 voice state 전달

7. Go 봇
   -> query를 URL 또는 ytsearch: 형태로 정규화
   -> Lavalink loadtracks 호출

8. Lavalink
   -> YouTube 플러그인으로 검색/트랙 로드
   -> 첫 번째 재생 가능 트랙 반환

9. Go 봇
   -> internal/player에 현재 곡 또는 대기열 반영
   -> 즉시 재생 대상이면 Lavalink play 요청

10. Lavalink
    -> Discord 음성 채널에서 실제 재생 시작

11. Go 봇
    -> interaction 응답 수정
    -> "Now playing" 또는 "Queued" 메시지 반환
```

### Lavalink YouTube 플러그인
- YouTube 검색어와 YouTube URL을 실제 재생 가능한 트랙 정보로 바꾼다.
- `/play`에서 전달한 검색어가 YouTube 검색 결과로 바뀌는 부분을 담당한다.

### Docker Compose
- 로컬 개발 환경에서 Lavalink 서버를 재현 가능하게 실행하는 역할을 한다.
- 로컬 PC마다 수동으로 자바 프로세스를 직접 띄우는 대신 같은 설정으로 Lavalink를 올리게 해 준다.

### `.env`
- Discord 토큰, 테스트 길드 ID, Lavalink 호스트/포트/비밀번호 같은 런타임 설정 값을 보관한다.
- 코드와 비밀값을 분리하는 역할을 한다.

### `internal/config`
- `.env`에서 넘어온 프로세스 환경 변수를 읽고 필수값을 검증한다.
- 잘못된 포트 값이나 누락된 키를 시작 시점에 바로 실패시키는 역할을 한다.

### `internal/lavalink`
- Lavalink REST API와 WebSocket API를 호출하는 어댑터 계층이다.
- 구현 기능:
  - `/version` 확인
  - WebSocket 세션 연결
  - 트랙 검색 및 로드
  - 재생 시작
  - 재생 중지
  - 플레이어 제거
  - 재생 종료 이벤트 수신
  - Discord voice state를 Lavalink 포맷으로 전달

### `internal/player`
- 길드별 재생 상태를 메모리에서 관리하는 도메인 계층이다.
- 구현 기능:
  - 현재 곡 저장
  - 대기열 FIFO 관리
  - 다음 곡 선택
  - 정지 시 큐 비우기
  - leave 시 상태 제거

### `cmd/bot/main.go`
- 실제 애플리케이션 조립 지점이다.
- `config`, `discordgo`, `lavalink`, `player`를 연결해 하나의 봇 프로세스로 만든다.
- 커맨드 핸들러와 이벤트 핸들러가 이 파일에 연결되어 있다.

## 현재 디렉터리 구조
핵심 파일과 폴더는 아래와 같다.

```text
cmd/bot/main.go
internal/config/
internal/lavalink/
internal/player/
infra/lavalink/
docs/
```

### `cmd/bot`
- 엔트리포인트가 있다.
- Discord 세션 생성, Lavalink 연결 확인, 슬래시 커맨드 등록, 이벤트 핸들러 연결을 담당한다.

### `internal/config`
- 환경 변수를 읽고 검증한다.
- Discord 토큰, 길드 ID, Lavalink 호스트/포트/비밀번호를 로드한다.

### `internal/lavalink`
- Lavalink v4와 통신하는 최소 클라이언트 계층이다.
- 포함 기능:
  - `/version` 확인
  - Lavalink WebSocket 세션 연결
  - 트랙 검색 및 로드
  - 플레이어 업데이트
  - 재생 중지
  - 플레이어 제거
  - Discord voice state 전달용 보조 상태 저장

### `internal/player`
- 길드별 재생 상태를 메모리로 관리한다.
- 현재 곡, 대기열, 연결 중인 음성 채널 ID를 보관한다.

### `infra/lavalink`
- 로컬 Lavalink 실행에 필요한 Compose 및 설정 파일이 있다.
- `application.yml`에는 Lavalink 서버 설정과 YouTube 플러그인 설정이 들어 있다.

### `docs`
- 운영 및 설명 문서를 둔다.
- 로컬 Lavalink 실행 가이드는 [lavalink-local.md](/Users/gimjinmyeong/Desktop/workspace/Github/HNMO-discord-bot/docs/lavalink-local.md#L1)에 있다.

## 패키지별 파일 설명
아래 표는 현재 저장소에서 자주 읽게 되는 파일을 패키지 단위로 정리한 것이다.

### `cmd/bot` (`package main`)
| 파일 | 역할 |
| --- | --- |
| [cmd/bot/main.go](/Users/gimjinmyeong/Desktop/workspace/Github/HNMO-discord-bot/cmd/bot/main.go#L1) | 봇의 엔트리포인트이자 조립 지점이다. 설정 로드, Discord 세션 생성, Lavalink 연결 확인, 슬래시 커맨드 등록, interaction 처리, voice state 동기화, 재생 제어 보조 함수까지 모두 포함한다. |

### `internal/config` (`package config`)
| 파일 | 역할 |
| --- | --- |
| [internal/config/config.go](/Users/gimjinmyeong/Desktop/workspace/Github/HNMO-discord-bot/internal/config/config.go#L1) | `.env`와 프로세스 환경 변수에서 런타임 설정을 읽고 검증한다. 필수 키 누락, 포트 형식 오류, 기본 로그 레벨 처리, Lavalink 주소/URL 조합 함수를 제공한다. |
| [internal/config/config_test.go](/Users/gimjinmyeong/Desktop/workspace/Github/HNMO-discord-bot/internal/config/config_test.go#L1) | 설정 로더의 정상 경로와 실패 경로를 검증한다. 기본 로그 레벨 적용, 필수 변수 누락 메시지, 잘못된 포트 처리 동작을 테스트한다. |

### `internal/lavalink` (`package lavalink`)
| 파일 | 역할 |
| --- | --- |
| [internal/lavalink/client.go](/Users/gimjinmyeong/Desktop/workspace/Github/HNMO-discord-bot/internal/lavalink/client.go#L1) | Lavalink REST API와 WebSocket API를 감싸는 핵심 클라이언트다. 버전 확인, 세션 연결, 트랙 로드, 플레이어 업데이트, 재생/정지/삭제, 이벤트 수신과 핸들러 연결을 담당한다. |
| [internal/lavalink/client_test.go](/Users/gimjinmyeong/Desktop/workspace/Github/HNMO-discord-bot/internal/lavalink/client_test.go#L1) | Lavalink 클라이언트의 HTTP 계약을 검증한다. `/version` 응답 처리, 인증 실패 메시지, 빈 응답 처리, `loadtracks` 검색 결과 선택 로직을 단위 테스트한다. |
| [internal/lavalink/voice_state.go](/Users/gimjinmyeong/Desktop/workspace/Github/HNMO-discord-bot/internal/lavalink/voice_state.go#L1) | Discord의 `VOICE_STATE_UPDATE`와 `VOICE_SERVER_UPDATE`를 합쳐 Lavalink에 넘길 완전한 voice state를 보관한다. 길드별 상태 저장, 대기자 깨우기, 채널 일치 검사, 상태 초기화를 맡는다. |
| [internal/lavalink/voice_state_test.go](/Users/gimjinmyeong/Desktop/workspace/Github/HNMO-discord-bot/internal/lavalink/voice_state_test.go#L1) | voice state 저장소의 동기화 동작을 검증한다. 두 이벤트가 모두 도착했을 때만 상태를 반환하는지, 다른 채널의 상태를 잘못 허용하지 않는지를 테스트한다. |

### `internal/player` (`package player`)
| 파일 | 역할 |
| --- | --- |
| [internal/player/manager.go](/Users/gimjinmyeong/Desktop/workspace/Github/HNMO-discord-bot/internal/player/manager.go#L1) | 길드별 재생 상태를 메모리에서 관리하는 도메인 계층이다. 현재 곡, 대기열, 활성 음성 채널을 저장하고 `enqueue`, `advance`, `stop`, `leave`, 스냅샷 조회를 제공한다. |
| [internal/player/manager_test.go](/Users/gimjinmyeong/Desktop/workspace/Github/HNMO-discord-bot/internal/player/manager_test.go#L1) | 플레이어 상태 전이 규칙을 검증한다. 첫 곡 즉시 재생, 다음 곡 큐 적재, `advance` 동작, `stop` 초기화, `leave` 시 상태 제거를 확인한다. |

## 보조 디렉터리 파일 설명
Go 패키지는 아니지만 로컬 실행과 문서 이해에 필요한 파일은 아래와 같다.

### `infra/lavalink`
| 파일 | 역할 |
| --- | --- |
| [infra/lavalink/compose.yml](/Users/gimjinmyeong/Desktop/workspace/Github/HNMO-discord-bot/infra/lavalink/compose.yml#L1) | 로컬 개발용 Lavalink 컨테이너 실행 정의다. 이미지, 포트 바인딩, 비밀번호 환경 변수, 설정 파일 마운트, 플러그인 디렉터리 마운트를 지정한다. |
| [infra/lavalink/application.yml](/Users/gimjinmyeong/Desktop/workspace/Github/HNMO-discord-bot/infra/lavalink/application.yml#L1) | Lavalink 서버 설정 파일이다. 서버 포트, 인증 비밀번호, 활성/비활성 소스, YouTube 플러그인 활성화와 검색 허용 정책을 정의한다. |

### `docs`
| 파일 | 역할 |
| --- | --- |
| [docs/project-overview.md](/Users/gimjinmyeong/Desktop/workspace/Github/HNMO-discord-bot/docs/project-overview.md#L1) | 프로젝트의 전체 구조, 런타임 흐름, 실행 방법, 현재 구현 범위를 설명하는 상위 개요 문서다. |
| [docs/lavalink-local.md](/Users/gimjinmyeong/Desktop/workspace/Github/HNMO-discord-bot/docs/lavalink-local.md#L1) | 로컬에서 Lavalink를 Docker Compose로 띄우고 상태를 확인하는 절차를 설명하는 운영 가이드다. |

## 런타임 구조
전체 구조는 아래처럼 나뉜다.

```text
Discord User
  -> Discord Slash Command
  -> Go Bot
  -> Lavalink
  -> Discord Voice Channel
```

역할 분리는 다음과 같다.

### Go Bot
- Discord interaction 수신
- 사용자 음성 채널 확인
- 길드별 큐 상태 관리
- Lavalink에 검색/재생/정지 명령 전달
- 결과를 Discord 메시지로 반환

### Lavalink
- Discord 음성 연결 수행
- YouTube 검색 및 트랙 로드
- 실제 재생 처리
- 트랙 종료 이벤트 전송

## 주요 동작 흐름
### 1. 봇 시작 흐름
1. `.env`에서 설정값을 읽는다.
2. Lavalink `/version`으로 기본 연결 가능 여부를 확인한다.
3. Discord 세션을 열고 `READY` 이벤트를 기다린다.
4. Lavalink WebSocket에 연결해 `sessionId`를 확보한다.
5. 슬래시 커맨드를 등록한다.
6. 이벤트 루프 상태로 대기한다.

### 2. `/play` 흐름
1. 사용자가 `/play query:<string>`를 호출한다.
2. 봇은 먼저 deferred interaction 응답을 보낸다.
3. 사용자가 현재 들어가 있는 음성 채널을 확인한다.
4. 봇이 아직 해당 길드에서 활성 상태가 아니면 Discord에 voice join 요청을 보낸다.
5. Discord의 `VOICE_STATE_UPDATE`, `VOICE_SERVER_UPDATE`를 받아 Lavalink용 voice state를 완성한다.
6. 검색어면 `ytsearch:`를 붙이고, URL이면 그대로 Lavalink `loadtracks`에 전달한다.
7. 현재 길드에서 재생 중인 곡이 없으면 바로 재생한다.
8. 재생 중인 곡이 있으면 큐 뒤에 추가한다.
9. 결과를 interaction 응답 수정으로 사용자에게 보여준다.

### 3. 자동 다음 곡 재생 흐름
1. Lavalink가 `TrackEndEvent`를 WebSocket으로 보낸다.
2. 봇이 길드 큐에서 다음 곡을 꺼낸다.
3. 다음 곡이 있으면 즉시 Lavalink에 재생 요청을 보낸다.
4. 없으면 현재 곡만 비우고 유휴 상태가 된다.

### 4. 제어 커맨드 흐름
#### `/skip`
- 현재 곡을 건너뛴다.
- 다음 곡이 있으면 즉시 재생한다.
- 다음 곡이 없으면 Lavalink에 stop 요청을 보낸다.

#### `/stop`
- 현재 재생과 대기열을 모두 비운다.
- Lavalink에도 stop 요청을 보낸다.

#### `/queue`
- 현재 곡과 대기열을 텍스트로 보여준다.

#### `/nowplaying`
- 현재 곡만 보여준다.

#### `/leave`
- 길드 플레이어 상태를 제거한다.
- Lavalink 플레이어를 destroy 한다.
- Discord 음성 채널에서 나가도록 요청한다.

## 상태 관리 방식
현재 상태는 메모리 기반이다.
즉, 프로세스를 재시작하면 큐와 현재 곡 정보는 사라진다.

길드별 상태는 [manager.go](/Users/gimjinmyeong/Desktop/workspace/Github/HNMO-discord-bot/internal/player/manager.go#L9)에서 관리한다.

저장하는 값:
- 현재 연결 중인 음성 채널 ID
- 현재 곡
- 대기열

이 방식의 장점:
- 단순하고 빠르다.
- 초기 구현과 로컬 검증에 적합하다.

이 방식의 한계:
- 영속성이 없다.
- 프로세스 재시작 시 상태 복구가 없다.

기본 Gateway 실행 모델은 단일 shard이다.
환경 변수를 생략하면 `SHARD_ID=0`, `SHARD_COUNT=1`로 동작한다.
여러 shard 프로세스를 실행하면 Discord guild 이벤트는 shard별로 분산되지만, 큐와 현재 곡 상태는 여전히 각 shard 프로세스의 메모리에만 저장된다.
따라서 shard 재시작 후 큐 복구나 다른 shard로의 상태 승계는 아직 제공하지 않는다.

## 실행 방법
기본 실행 절차는 아래와 같다.

### 1. `.env` 준비
루트에 `.env` 파일을 두고 아래 값을 채운다.

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

### 2. Lavalink 실행

```bash
docker compose -f infra/lavalink/compose.yml up -d
```

정상 확인:

```bash
docker compose -f infra/lavalink/compose.yml logs --tail=200 lavalink
curl -sS -H "Authorization: ${LAVALINK_PASSWORD}" http://127.0.0.1:2333/version
```

상세한 로컬 실행 절차는 [lavalink-local.md](/Users/gimjinmyeong/Desktop/workspace/Github/HNMO-discord-bot/docs/lavalink-local.md#L1)를 참고한다.

### 3. 봇 실행

```bash
set -a
source .env
set +a
go run ./cmd/bot
```

정상 로그 예시:
- `connected to Lavalink 4.2.2`
- `discord gateway shard configured: shard_id=0 shard_count=1`
- `logged in as ...`
- `Lavalink websocket session ... is ready`
- `bot is running. press Ctrl+C to exit.`

### 4. 여러 shard로 실행
`SHARD_COUNT`는 모든 프로세스에서 같은 값이어야 하고, `SHARD_ID`는 `0`부터 `SHARD_COUNT - 1`까지 프로세스마다 다르게 둔다.
slash command 등록은 한 프로세스에서만 켜고, 다중 shard 운영에서는 종료 시 command 삭제를 끈다.

```bash
SHARD_ID=0 SHARD_COUNT=3 DISCORD_COMMAND_REGISTRATION_ENABLED=true DISCORD_COMMAND_CLEANUP_ENABLED=false METRICS_ADDR=127.0.0.1:2112 go run ./cmd/bot
SHARD_ID=1 SHARD_COUNT=3 DISCORD_COMMAND_REGISTRATION_ENABLED=false DISCORD_COMMAND_CLEANUP_ENABLED=false METRICS_ADDR=127.0.0.1:2113 go run ./cmd/bot
SHARD_ID=2 SHARD_COUNT=3 DISCORD_COMMAND_REGISTRATION_ENABLED=false DISCORD_COMMAND_CLEANUP_ENABLED=false METRICS_ADDR=127.0.0.1:2114 go run ./cmd/bot
```

## 수동 검증 절차
현재 구현 상태를 확인하려면 Discord 서버에서 아래 순서로 테스트하면 된다.

1. 봇이 초대된 테스트 서버에 들어간다.
2. 음성 채널에 먼저 입장한다.
3. `/play query:<검색어 또는 유튜브 URL>` 실행
4. `/queue` 실행
5. `/nowplaying` 실행
6. `/skip` 실행
7. `/stop` 실행
8. `/leave` 실행

정상 기대 결과:
- `/play`는 첫 곡을 재생하거나 큐에 추가한다.
- `/queue`는 현재 곡과 대기열을 보여준다.
- `/skip`은 다음 곡으로 넘어간다.
- `/stop`은 재생과 큐를 비운다.
- `/leave`는 음성 채널에서 나간다.

## 현재 구현 범위
현재 기준으로 구현된 범위:
- Lavalink 로컬 실행
- Go 앱의 Lavalink 시작 시 연결 검증
- Discord slash command 등록
- `/ping`
- `/play`
- `/skip`
- `/stop`
- `/queue`
- `/nowplaying`
- `/leave`
- 길드별 메모리 큐
- 설정 가능한 Discord Gateway shard
- 트랙 종료 시 자동 다음 곡 재생

아직 없는 것:
- `/pause`, `/resume`
- 플레이리스트 전체 제어
- 영속 저장소
- 운영용 모니터링
- 다중 노드 Lavalink 전략
- shard startup coordinator와 대규모 공개 배포용 자동 샤딩 전략

## 운영 시 주의사항
### Lavalink가 먼저 떠 있어야 한다
봇은 시작 시점에 Lavalink `/version`과 WebSocket 세션 연결을 확인한다.
따라서 Lavalink가 꺼져 있으면 봇도 시작하지 않는다.

### `.env`와 Lavalink 설정 비밀번호는 같아야 한다
`LAVALINK_PASSWORD`가 다르면 인증 오류가 난다.

### 현재 큐는 메모리 기반이다
봇이 재시작되면 현재 곡과 대기열 정보는 사라진다.

### 다중 shard에서는 command 등록을 한 프로세스만 수행한다
`DISCORD_COMMAND_REGISTRATION_ENABLED=true`는 한 프로세스에만 설정한다.
다중 shard 운영에서는 `DISCORD_COMMAND_CLEANUP_ENABLED=false`로 두어 한 shard 종료가 slash command를 삭제하지 않게 한다.

### 같은 길드에서 다른 음성 채널로 동시에 사용하지 않는다
현재 구현은 길드별 단일 활성 음성 채널을 기준으로 동작한다.
이미 다른 채널에서 활성 상태이면 새 `/play` 요청을 막는다.

## 관련 문서
- 로컬 Lavalink 실행 가이드: [lavalink-local.md](/Users/gimjinmyeong/Desktop/workspace/Github/HNMO-discord-bot/docs/lavalink-local.md#L1)
- 진입점: [main.go](/Users/gimjinmyeong/Desktop/workspace/Github/HNMO-discord-bot/cmd/bot/main.go#L1)
- Lavalink 클라이언트: [client.go](/Users/gimjinmyeong/Desktop/workspace/Github/HNMO-discord-bot/internal/lavalink/client.go#L1)
- 길드 상태 관리: [manager.go](/Users/gimjinmyeong/Desktop/workspace/Github/HNMO-discord-bot/internal/player/manager.go#L1)
