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

`hef`는 결과가 실제로 짧아질 때만 패턴 필터를 적용합니다. 주요 케이스는 아래와 같습니다.

### 경로와 검색 출력

검색 결과는 파일별로 묶고, 파일 목록은 디렉터리별로 묶습니다.

```text
C:/Work/Project/Assets/Foo.cs:10: TODO
C:/Work/Project/Assets/Foo.cs:48: FIXME
C:/Work/Project/Assets/Bar.cs:31: TODO
```

```text
$root=C:/Work/Project/Assets
$root/Foo.cs (2)
  10: TODO
  48: FIXME
$root/Bar.cs (1)
  31: TODO
```

### 반복 텍스트

같은 줄 반복과 연속 번호는 제자리에서 접습니다.

```text
loading
loading
loading
file_001.tmp
file_002.tmp
file_003.tmp
```

```text
loading (x3)
file_001..003.tmp (3 lines)
```

### 공통 prefix와 긴 토큰

긴 공통 prefix와 반복되는 ID는 짧은 별칭으로 바뀝니다.

```text
Namespace.Project.Feature.Alpha
Namespace.Project.Feature.Beta
created 550e8400-e29b-41d4-a716-446655440000
updated 550e8400-e29b-41d4-a716-446655440000
```

```text
$prefix1=Namespace.Project.Feature.
$t1=550e8400-e29b-41d4-a716-446655440000
$prefix1Alpha
$prefix1Beta
created $t1
updated $t1
```

### 거대한 출력

그래도 출력이 크면 앞부분, error 같은 중요 줄이나 `-focus` 키워드 줄, 끝부분만 남기고 생략량을 표시합니다.
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
