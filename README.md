# Human Eye Filter

Reads command output the way a human eye would, so an LLM doesn't burn context on redundant lines.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## Why

Humans skim. Faced with a wall of repetitive, redundant output, a person's eye jumps to what matters and ignores the rest. An LLM can't — it reads every token, and redundant output silently burns its context and its budget. Human Eye Filter reads command output the way a human eye would: it keeps the high-signal lines, collapses duplicates, groups repeated paths, samples bulk JSON, and reports how much it dropped — before the output ever reaches the model. Short output passes through untouched, because small exact results still matter.

It is not a command rewriter and it is not a per-tool adapter. The command stays flexible; its output passes through one reducer that recognizes a small set of common shapes and shrinks them consistently.

## Install

`go install`:

```sh
go install github.com/youngwoocho02/human-eye-filter/cmd/humaneye@latest
```

Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/youngwoocho02/human-eye-filter/master/install.ps1 | iex
```

Linux / macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/youngwoocho02/human-eye-filter/master/install.sh | sh
```

Build from source:

```sh
go build -o bin/humaneye ./cmd/humaneye
```

## Quick Start

Pipe any output into `humaneye`, or let it run the command for you:

```sh
rg -n --glob '*.cs' 'ContentAvailability' Assets | humaneye --mode grep
humaneye --mode json --max-chars 8000 < result.json
humaneye --mode unity < Editor.log
humaneye run -- rg -n --glob '*.cs' 'Foo' Assets
humaneye run-auto -- git status
```

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

## Subcommands

### `run`

Runs a command, reduces the combined stdout/stderr, and returns the child's exit code.

```sh
humaneye run -- rg -n --glob '*.cs' 'Foo' Assets
```

### `run-auto`

Same as `run`, but short output passes through raw. It reduces only when the output crosses `--auto-min-lines` (40) or `--auto-min-chars` (4000). This is the form to use in hooks: small exact results stay intact, large dumps get reduced.

Short output — passes through untouched:

```sh
$ humaneye run-auto -- git rev-parse --short HEAD
a1b2c3d
```

Large output — reduced, with a report of how much was dropped:

```sh
$ humaneye run-auto -- rg -n --glob '*.cs' 'Foo' Assets
$root=Assets/Accelix/Scripts
$root/Demo/DemoScopeService.cs (3)
  42: ...
  88: ...
... <reduced from 612 to 41 lines>
```

### `hook powershell`

Prints PowerShell helper functions to stdout:

```powershell
humaneye hook powershell
```

The generated helpers:

- `eye <command>` — threshold-based generic wrapper.
- `eye-rg ...` — `rg` output with the grep reducer hint.
- `eye-git ...` — Git output with the SCM reducer hint.
- `eye-cm ...` — PlasticSCM output with the SCM reducer hint.
- `eye-unity ...` — Unity CLI output with the Unity log reducer hint.

### `hook claude` / `hook codex`

The built-in `PreToolUse` hook handler. Reads the hook event JSON on stdin and prints the rewrite JSON on stdout, wrapping the Bash-tool command as `humaneye run-auto -- bash -c '<cmd>'` (using its own absolute path). Claude Code and OpenAI Codex share one schema, so the two subcommands behave identically. It fails open (emits nothing) on bad input, background commands, and already-wrapped commands, so it can never block the Bash tool. This is what a config registers as the hook command — no external script needed.

### `hook install` / `hook uninstall`

Registers (or removes) the hook in each agent's config, idempotently:

```sh
humaneye hook install --agent all      # claude + codex + opencode
humaneye hook install --agent claude
humaneye hook uninstall --agent all
```

- **claude** — adds a `PreToolUse` Bash hook calling `humaneye hook claude` to `~/.claude/settings.json`.
- **codex** — appends a `[[hooks.PreToolUse]]` block calling `humaneye hook codex` to `~/.codex/config.toml`.
- **opencode** — writes a `tool.execute.before` plugin and registers it in `~/.config/opencode/opencode.json`.

## Options

| Flag                | Default   | Description                                                       |
| ------------------- | --------- | ----------------------------------------------------------------- |
| `-mode`             | `auto`    | Reducer mode: `auto`, `text`, `json`, `paths`, `grep`, `unity`, `scm`. |
| `-max-lines`        | `160`     | Maximum output lines.                                             |
| `-max-chars`        | `12000`   | Maximum output characters.                                       |
| `-max-input-bytes`  | `4194304` | Maximum raw input bytes read before reducing.                    |
| `-auto-min-lines`   | `40`      | Minimum raw lines before `run-auto` reduces.                     |
| `-auto-min-chars`   | `4000`    | Minimum raw characters before `run-auto` reduces.                |
| `-tool`             | _(none)_  | Tool hint, e.g. `rg`, `grep`, `git`, `cm`, `unity-cli`, `unity-scanner`. |
| `-focus`            | _(none)_  | Comma-separated keywords to keep before generic samples.         |
| `-keep`             | `error`   | Priority to keep: `error`, `warning`, `path`, `all`.             |
| `-raw-on-fail`      | `true`    | Print raw input if reduction fails.                              |
| `-version`          | `false`   | Print version and exit (also `version` subcommand).              |

`-tool` is a hint, not a hard parser: it maps a tool to the nearest shape reducer, and unknown tools still work through `auto`. `-focus` keeps project terms (e.g. `DemoScope`, `Addressables`) that are not errors but still matter.

## Agent Integration

The intended use is a hook that wraps the agent's Bash-tool commands so their output is reduced before it reaches the model. Set it up in one command:

```sh
humaneye hook install --agent all
```

- **Claude Code** and **OpenAI Codex** use a `PreToolUse` hook (same hook JSON schema) that rewrites the Bash-tool command to `humaneye run-auto -- bash -c '<cmd>'`. The hook handler is built in (`humaneye hook claude` / `humaneye hook codex`).
- **OpenCode** uses a plugin `tool.execute.before` hook to do the same.

Because the wrapper uses `run-auto`, short output stays raw and only large output is reduced. Only the Bash tool is covered — the Read and Grep tools and the PowerShell tool are not wrapped, so their results are unaffected.

## Development

```sh
gofmt -w .
go test ./...
go build ./...
```

## License

MIT. See [LICENSE](LICENSE).
