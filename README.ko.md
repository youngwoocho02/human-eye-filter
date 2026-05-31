# Human Eye Filter

[English](README.md) | [Korean](README.ko.md) | [Japanese](README.ja.md)

> AI에 들어가기 전에 명령 출력을 읽기 쉬운 크기로 접어 주는 파이프 필터.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**모드 선택 없음. 도구별 어댑터 없음. 그냥 `| hef`만 붙이면 됩니다.**

## 왜 필요한가

사람은 수천 글자 출력에서 필요한 줄만 빠르게 훑어봅니다. 하지만 AI 모델은 반복 경로, 중복 줄, 긴 ID, 거대한 덤프까지 모두 토큰으로 받습니다. `hef`는 사람이 건너뛰는 반복을 접어 토큰 낭비를 줄이고, 출력은 계속 읽을 수 있게 유지합니다.

`hef`는 명령 실행기가 아닙니다. 원래 명령은 그대로 두고, stdin으로 들어온 출력만 줄여 stdout으로 내보냅니다.

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

### 검색 결과

긴 `path:line:text` 출력은 파일별로 묶입니다.

```text
C:/Work/Project/Assets/Foo.cs:10: TODO: split setup
C:/Work/Project/Assets/Foo.cs:48: FIXME: stale state
C:/Work/Project/Assets/Bar.cs:31: TODO: wire label
```

```text
$root=C:/Work/Project/Assets
$root/Foo.cs (2)
  10: TODO: split setup
  48: FIXME: stale state
$root/Bar.cs (1)
  31: TODO: wire label
```

### 파일 목록

긴 파일 목록은 전체 경로를 반복하지 않고 디렉터리 형태를 보존합니다.

```text
C:/Work/Project/Assets/CookStation/Blender/A.cs
C:/Work/Project/Assets/CookStation/Blender/B.cs
C:/Work/Project/Assets/CookStation/Brewer/C.cs
```

```text
$root=C:/Work/Project/Assets/CookStation
$root/Blender/ (2)
  A.cs
  B.cs
$root/Brewer/ (1)
  C.cs
```

### 반복

같은 줄, 연속 번호, 반복되는 긴 토큰은 제자리에서 접습니다.

```text
loading
loading
loading
file_001.tmp
file_002.tmp
file_003.tmp
created 550e8400-e29b-41d4-a716-446655440000
updated 550e8400-e29b-41d4-a716-446655440000
```

```text
$t1=550e8400-e29b-41d4-a716-446655440000
loading (x3)
file_001..003.tmp (3 lines)
created $t1
updated $t1
```

출력이 너무 크면 앞부분, 중요한 줄, 끝부분만 남기고 생략량을 표시합니다.

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
