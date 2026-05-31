# Human Eye Filter

Reads command output like a human eye before it reaches an AI: collapses repeated lines, numeric ranges, common prefixes, repeated long tokens, and oversized output.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## Why

People can scan thousands of characters and jump to the useful lines. AI models receive that same output as tokens, including every repeated path, duplicate error, giant JSON array, and broad search dump. Human Eye Filter reduces that waste before the output reaches the model.

It is not a command runner and not a per-tool adapter. The command stays flexible; `hef` only filters the output it receives.

## Install

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/youngwoocho02/human-eye-filter/master/install.ps1 | iex
```

Linux / macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/youngwoocho02/human-eye-filter/master/install.sh | sh
```

The installer downloads the latest GitHub Release binary and adds the install directory to `PATH`. After installing, run commands as `hef ...`.

## What It Does

AI models spend tokens on every character in command output. `hef` keeps the output readable, but folds the parts a person would mentally skip: repeated lines, repeated paths, numeric runs, long shared prefixes, repeated IDs, and huge tails.

Use it by adding one pipe:

```sh
some-command | hef
```

### Search results

Long `path:line:text` output becomes one group per file.

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

### File lists

Large file lists keep the directory shape instead of repeating the full prefix.

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

### Repetition

Repeated lines, numeric runs, and long repeated tokens are folded in place.

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

For very large output, `hef` keeps the head, important lines, and tail, then reports how much was omitted.
## Update

```sh
hef update
hef update --check
```

`hef` checks for a newer GitHub Release at most once per hour and prints a short notice when an update is available. Update checks are cached under the user profile and ignored silently when offline.

## Quick Start

Pipe output into `hef`:

```sh
some-command | hef
rg -n --glob '*.cs' 'ContentAvailability' Assets | hef
```

## Agent Setup

Agents should keep running normal shell commands. Their shell hook or plugin only appends `| hef`.

```sh
hef setup --agent all
```

Supported agents:

- `claude` writes a Claude Code `PreToolUse` hook script.
- `codex` writes a Codex `PreToolUse` hook script.
- `opencode` writes an OpenCode `tool.execute.before` plugin.

Generated files:

- Claude Code: `~/.claude/hooks/hef-pipe.ps1`, registered in `~/.claude/settings.json`
- OpenAI Codex: `~/.codex/hooks/hef-pipe.ps1`, registered in `~/.codex/config.toml`
- OpenCode: `~/.config/opencode/plugins/hef.ts`, registered in `~/.config/opencode/opencode.json`

Remove the generated setup:

```sh
hef setup --agent all --remove
```

Only shell-command tools are covered. Direct file-read, browser, image, and other non-shell tools are not piped through `hef`.

## Options

| Flag                | Default   | Description                                                       |
| ------------------- | --------- | ----------------------------------------------------------------- |
| `-max-lines`        | `160`     | Maximum output lines.                                             |
| `-max-chars`        | `12000`   | Maximum output characters.                                       |
| `-max-input-bytes`  | `4194304` | Maximum raw input bytes read before reducing.                    |
| `-focus`            | _(none)_  | Comma-separated keywords to keep before generic samples.         |
| `-raw-on-fail`      | `true`    | Print raw input if reduction fails.                              |
| `-version`          | `false`   | Print version and exit.                                           |

Most runs should not need flags. Use them only when the automatic output is too broad, too short, or missing a keyword you care about.

When `hef` reports `output limit reached` or `input limit reached`, narrow the original command and rerun it with more specific paths, globs, filters, or counts.

## Development

```sh
gofmt -w .
go test ./...
go build ./...
```

## License

MIT. See [LICENSE](LICENSE).
