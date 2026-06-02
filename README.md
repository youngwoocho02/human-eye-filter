# Human Eye Filter

[English](README.md) | [Korean](README.ko.md) | [Japanese](README.ja.md)

> A pipe filter that shrinks command output before it reaches an AI.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**No mode selection. No tool adapter. Just pipe output into `hef`.**

## Why

People can scan thousands of characters and jump to the useful lines. AI models receive every repeated path, duplicate line, long ID, and oversized dump as tokens. `hef` folds that waste while keeping the output readable.

It is not a command runner. The original command stays unchanged; `hef` only filters stdin and writes reduced output to stdout.

## Compared to RTK

RTK is a command proxy. Its agent hooks rewrite commands such as `git status` into `rtk git status`, then RTK routes that command through command-specific filters.

`hef` is a plain output filter. It does not replace the command, does not need per-tool command wrappers, and does not ask the user to pick a mode. The original command runs normally; `hef` only folds repeated patterns in stdout.

```sh
# RTK-style flow
agent command -> hook rewrite -> rtk <command> -> command-specific filter

# HEF-style flow
agent command -> original command runs -> stdout | hef -> pattern filters
```
## Install

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/youngwoocho02/human-eye-filter/master/install.ps1 | iex
```
Linux / macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/youngwoocho02/human-eye-filter/master/install.sh | sh
```
## Quick Start

Register the agent hook once. After that, agents are warned when a shell command does not use `hef`.

```sh
hef setup --agent all
```
Use `hef` on commands that may produce long output:

```sh
some-command | hef
```
For rare cases where piping through `hef` is not appropriate, add an explicit opt-out marker such as `[no-hef]`, `--no-hef`, or `# no-hef`.
## What Changes

`hef` runs seven collapsing passes, applying each only when it makes the result shorter, then appends a one-line summary footer. Every example below is real `hef` output.

### 1. Repeated lines

Consecutive identical lines fold into one with a count.

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

### 2. Sequential ranges

Runs of consecutively numbered files or lines collapse into a single range.

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

### 3. Grep-style results grouped by file

`path:line:text` hits group under each file, the shared workspace path is hoisted into a `$root` alias, and long hit lists are capped.

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

### 4. File lists grouped by directory

A flat list of files groups under each directory, again sharing a `$root` alias.

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

### 5. Common prefixes

When many lines share a long prefix (20+ chars), it is hoisted into a `$prefix` alias.

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

### 6. Dictionary tokens

Long repeated strings (24+ chars: GUIDs, URLs, paths) are defined once and aliased everywhere.

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

### 7. Bounded sampling

When output exceeds the line budget (`-max-lines`), `hef` keeps a head sample and a tail sample, drops the middle, and rescues important lines (`error`, `fatal`, `panic`, ...) that would otherwise be lost. Running `hef -max-lines 10` over these 15 lines, whose 8th line is an error:

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

The `ERROR` line is rescued into the `## important` block, so a failure never gets silently dropped.

Before any pass runs, input is normalized: ANSI color escapes, NUL bytes, and CR/CRLF line endings are stripped. Every run also appends a `--- hef summary ---` footer reporting which filters fired and the line/character reduction.
## Update

```sh
hef update
hef update --check
```
## Agent Setup

`hef setup` writes the hook or plugin files for supported agents.

Supported agents:

- `claude` writes a Claude Code `PreToolUse` hook script.
- `codex` writes a Codex `PreToolUse` hook script.
- `opencode` writes an OpenCode `tool.execute.before` plugin.

The generated hook/plugin does not rewrite commands. It blocks the tool call with a warning unless the command already uses `hef` or includes an explicit opt-out marker.

Remove the generated setup:

```sh
hef setup --agent all --remove
```
Only shell-command tools are covered. Direct file-read, browser, image, and other non-shell tools are not piped through `hef`.

## Options

| Flag                | Default   | Description                                    |
| ------------------- | --------- | ---------------------------------------------- |
| `-max-lines`        | `160`     | Maximum output lines.                          |
| `-max-chars`        | `12000`   | Maximum output characters.                     |
| `-max-input-bytes`  | `4194304` | Maximum raw input bytes read before reducing.  |
| `-focus`            | _(none)_  | Keywords to keep before generic samples.       |
| `-raw-on-fail`      | `true`    | Print raw input if reduction fails.            |
| `-version`          | `false`   | Print version and exit.                        |

Most runs should not need flags.

## Development

```sh
gofmt -w .
go test ./...
go build ./...
```
## License

MIT. See [LICENSE](LICENSE).
