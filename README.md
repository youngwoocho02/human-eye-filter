# Human Eye Filter

Reads command output like a human eye: keeps high-signal lines, collapses repeated noise, groups paths, samples large JSON, and reports what was omitted before the output reaches an LLM.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## Why

LLMs read every token in command output. Repeated paths, duplicate errors, giant JSON arrays, and broad search dumps silently burn context. Human Eye Filter is a pipe filter: command output goes in through stdin, reduced output comes out through stdout.

It is not a command runner and not a per-tool adapter. The command stays flexible; `hef` only reduces the output it receives.

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

## Update

```sh
hef update
hef update --check
```

`hef` checks for a newer GitHub Release at most once per hour and prints a short notice when an update is available. Update checks are cached under the user profile and ignored silently when offline.

## Quick Start

Pipe output into `hef`:

```sh
rg -n --glob '*.cs' 'ContentAvailability' Assets | hef --mode grep
hef --mode json --max-chars 8000 < result.json
hef --mode unity < Editor.log
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

## Modes

`--mode` selects a reducer. Default is `auto`, which detects the input shape.

| Mode    | Keeps                                                                  |
| ------- | --------------------------------------------------------------------- |
| `auto`  | Detects the input shape and picks a reducer.                          |
| `text`  | Collapses duplicate lines; keeps error-like lines first.              |
| `grep`  | Groups `path:line:text` matches by file.                              |
| `paths` | Groups file paths by directory.                                       |
| `json`  | Keeps object structure and samples long arrays.                       |
| `unity` | Keeps Unity error, exception, shader, and build lines first.          |
| `scm`   | Splits real content changes from checkout-only/branch noise.          |

Long repeated path prefixes are shortened automatically, and SCM output is split into sections so checkout-only noise and real content changes are easy to separate.

## Options

| Flag                | Default   | Description                                                       |
| ------------------- | --------- | ----------------------------------------------------------------- |
| `-mode`             | `auto`    | Reducer mode: `auto`, `text`, `json`, `paths`, `grep`, `unity`, `scm`. |
| `-max-lines`        | `160`     | Maximum output lines.                                             |
| `-max-chars`        | `12000`   | Maximum output characters.                                       |
| `-max-input-bytes`  | `4194304` | Maximum raw input bytes read before reducing.                    |
| `-tool`             | _(none)_  | Tool hint, e.g. `rg`, `grep`, `git`, `cm`, `unity-cli`, `unity-scanner`. |
| `-focus`            | _(none)_  | Comma-separated keywords to keep before generic samples.         |
| `-keep`             | `error`   | Priority to keep: `error`, `warning`, `path`, `all`.             |
| `-raw-on-fail`      | `true`    | Print raw input if reduction fails.                              |
| `-version`          | `false`   | Print version and exit.                                           |

When `hef` reports `output limit reached` or `input limit reached`, narrow the original command and rerun it with more specific paths, globs, filters, or counts.

## Development

```sh
gofmt -w .
go test ./...
go build ./...
```

## License

MIT. See [LICENSE](LICENSE).
