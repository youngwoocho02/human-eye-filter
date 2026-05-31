# Human Eye Filter

[English](README.md) | [Korean](README.ko.md) | [Japanese](README.ja.md)

> AI에 들어가기 전에 명령 출력을 읽기 쉬운 크기로 접어 주는 파이프 필터.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**모드 선택 없음. 도구별 어댑터 없음. 그냥 `| hef`만 붙이면 됩니다.**

## 왜 필요한가

사람은 수천 글자 출력에서 필요한 줄만 빠르게 훑어봅니다. 하지만 AI 모델은 반복 경로, 중복 줄, 긴 ID, 거대한 덤프까지 모두 토큰으로 받습니다. `hef`는 사람이 건너뛰는 반복을 접어 토큰 낭비를 줄이고, 출력은 계속 읽을 수 있게 유지합니다.

`hef`는 명령 실행기가 아닙니다. 원래 명령은 그대로 두고, stdin으로 들어온 출력만 줄여 stdout으로 내보냅니다.

## RTK와의 차이

RTK는 command proxy입니다. 에이전트 훅이 `git status` 같은 명령을 `rtk git status`로 바꾸고, RTK가 그 명령을 명령별 필터로 처리합니다.

`hef`는 단순 출력 필터입니다. 명령을 대체하지 않고, 도구별 command wrapper도 필요 없고, 사용자가 mode를 고르지 않아도 됩니다. 원래 명령은 그대로 실행되고, `hef`는 stdout에 반복되는 패턴만 접습니다.

```sh
# RTK 방식
agent command -> hook rewrite -> rtk <command> -> command-specific filter

# HEF 방식
agent command -> original command runs -> stdout | hef -> pattern filters
```
## 설치

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/youngwoocho02/human-eye-filter/master/install.ps1 | iex
```
Linux / macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/youngwoocho02/human-eye-filter/master/install.sh | sh
```
## 빠른 사용

에이전트 훅을 한 번 등록하면 끝입니다. 이후 에이전트는 평소처럼 셸 명령을 실행하고, `hef`가 자동으로 붙습니다.

```sh
hef setup --agent all
```
직접 필터링하고 싶을 때는 수동 파이프도 그대로 쓸 수 있습니다.

```sh
some-command | hef
```
## 어떻게 줄어드나

`hef`는 7개의 접기 패스를 실행하며, 각 패스는 결과가 실제로 짧아질 때만 적용되고, 마지막에 한 줄짜리 요약 푸터를 붙입니다. 아래 예시는 모두 실제 `hef` 출력입니다.

### 1. 반복되는 줄

연속으로 동일한 줄은 횟수와 함께 한 줄로 접힙니다.

```text
compiling module foo
warning: unused variable x
warning: unused variable x
warning: unused variable x
warning: unused variable x
done
```

```text
compiling module foo
warning: unused variable x (x4)
done

--- hef summary ---
filters=RepeatedLine lines=6->3 chars=133->57
```

### 2. 연속 번호 범위

연속 번호가 매겨진 파일이나 줄의 연속은 하나의 범위로 접힙니다.

```text
created file_001.tmp
created file_002.tmp
created file_003.tmp
created file_004.tmp
created file_005.tmp
created file_006.tmp
```

```text
created file_001..006.tmp (6 lines)

--- hef summary ---
filters=SequentialRange lines=6->1 chars=125->35
```

### 3. Grep 스타일 결과 파일별 그룹화

`path:line:text` 형태의 결과는 파일별로 묶이고, 공유하는 작업 공간 경로는 `$root` 별칭으로 끌어올려지며, 긴 결과 목록은 일정 개수에서 잘립니다.

```text
C:/WorkSpace/proj/src/app/main.go:10:func main() {
C:/WorkSpace/proj/src/app/main.go:25:return
C:/WorkSpace/proj/src/app/main.go:41:log.Fatal(err)
C:/WorkSpace/proj/src/app/main.go:58:os.Exit(1)
C:/WorkSpace/proj/src/app/main.go:77:}
C:/WorkSpace/proj/src/db/store.go:12:type Store struct {
C:/WorkSpace/proj/src/db/store.go:30:func Open() {
```

```text
$root=C:/WorkSpace/proj/src
$root/app/main.go (5)
  10: func main() {
  25: return
  41: log.Fatal(err)
  58: os.Exit(1)
  ... <1 more>
$root/db/store.go (2)
  12: type Store struct {
  30: func Open() {

--- hef summary ---
filters=PathLineGroup lines=7->10 chars=341->203
```

### 4. 디렉터리별 파일 목록 그룹화

평탄한 파일 목록은 디렉터리별로 묶이고, 역시 `$root` 별칭을 공유합니다.

```text
C:/WorkSpace/proj/internal/core/router.go
C:/WorkSpace/proj/internal/core/handler.go
C:/WorkSpace/proj/internal/core/config.go
C:/WorkSpace/proj/internal/core/logger.go
C:/WorkSpace/proj/internal/core/server.go
C:/WorkSpace/proj/internal/core/client.go
C:/WorkSpace/proj/internal/api/routes.go
C:/WorkSpace/proj/internal/api/middleware.go
```

```text
$root=C:/WorkSpace/proj/internal
$root/api/ (2)
  middleware.go
  routes.go
$root/core/ (6)
  client.go
  config.go
  handler.go
  logger.go
  router.go
  server.go

--- hef summary ---
filters=DirectoryGroup lines=8->11 chars=338->164
```

### 5. 공통 prefix

여러 줄이 긴 공통 prefix(20자 이상)를 공유하면 `$prefix` 별칭으로 끌어올립니다.

```text
/var/lib/myapp/cache/session-alpha.bin
/var/lib/myapp/cache/session-beta.log
/var/lib/myapp/cache/session-gamma.tmp
/var/lib/myapp/cache/session-delta.dat
```

```text
$prefix1=/var/lib/myapp/cache/session-
$prefix1alpha.bin
$prefix1beta.log
$prefix1gamma.tmp
$prefix1delta.dat

--- hef summary ---
filters=CommonPrefix lines=4->5 chars=154->109
```

### 6. Dictionary 토큰

길고 반복되는 문자열(24자 이상: GUID, URL, 경로)은 한 번만 정의하고 모든 위치에서 별칭으로 치환합니다.

```text
ref 550e8400-e29b-41d4-a716-446655440000 start
mid 550e8400-e29b-41d4-a716-446655440000 more
end 550e8400-e29b-41d4-a716-446655440000 tail
```

```text
$t1=550e8400-e29b-41d4-a716-446655440000
ref $t1 start
mid $t1 more
end $t1 tail

--- hef summary ---
filters=DictionaryToken lines=3->4 chars=138->80
```

### 7. 한도 기반 샘플링

출력이 줄 한도(`-max-lines`)를 초과하면 `hef`는 머리 부분 샘플과 꼬리 부분 샘플을 남기고 중간을 버리되, 사라지면 안 되는 중요한 줄(`error`, `fatal`, `panic` 등)은 건져냅니다. 8번째 줄이 에러인 다음 15줄에 `hef -max-lines 10`을 적용하면:

```text
starting build pipeline
loading dependency graph
resolving module versions
compiling core package
linking shared objects
generating documentation
optimizing image assets
ERROR undefined symbol in audio engine
running unit tests
measuring code coverage
packaging release archive
signing platform binaries
uploading to artifact store
notifying release channel
cleaning temporary workspace
```

```text
## head
starting build pipeline
loading dependency graph
resolving module versions
compiling core package
linking shared objects
generating documentation
## important
ERROR undefined symbol in audio engine
## tail
measuring code coverage
packaging release archive
signing platform binaries
uploading to artifact store
notifying release channel
cleaning temporary workspace
... <omitted 3 lines>

--- hef summary ---
filters=BoundedSample lines=15->17 chars=386->394
```

`ERROR` 줄이 `## important` 블록으로 건져올려져, 실패가 조용히 사라지지 않습니다.

어떤 패스가 돌기 전에 입력은 정규화됩니다. ANSI 색상 이스케이프, NUL 바이트, CR/CRLF 줄바꿈이 제거됩니다. 또한 매 실행마다 어떤 필터가 동작했고 줄/문자 수가 얼마나 줄었는지 알려주는 `--- hef summary ---` 푸터가 출력 끝에 붙습니다.
## 업데이트

```sh
hef update
hef update --check
```
## 에이전트 설정

`hef setup`은 지원 에이전트의 훅이나 플러그인 파일을 작성합니다.

지원 에이전트:

- `claude`: Claude Code `PreToolUse` 훅 스크립트를 씁니다.
- `codex`: OpenAI Codex `PreToolUse` 훅 스크립트를 씁니다.
- `opencode`: OpenCode `tool.execute.before` 플러그인을 씁니다.

제거:

```sh
hef setup --agent all --remove
```
셸 명령 도구만 대상입니다. 직접 파일 읽기, 브라우저, 이미지, 기타 비셸 도구 출력은 `hef`를 거치지 않습니다.

## 옵션

| Flag                | Default   | Description |
| ------------------- | --------- | ----------- |
| `-max-lines`        | `160`     | 최대 출력 줄 수 |
| `-max-chars`        | `12000`   | 최대 출력 문자 수 |
| `-max-input-bytes`  | `4194304` | 줄이기 전에 읽을 최대 원본 바이트 |
| `-focus`            | _(none)_  | 우선 보존할 키워드 |
| `-raw-on-fail`      | `true`    | 줄이기 실패 시 원본 출력 |
| `-version`          | `false`   | 버전 출력 |

대부분의 실행에는 옵션이 필요 없습니다.

## 개발

```sh
gofmt -w .
go test ./...
go build ./...
```
## 라이선스

MIT. [LICENSE](LICENSE)를 참고하세요.
