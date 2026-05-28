# token-sieve

Token-aware output reducer for agents.

`token-sieve` accepts arbitrary command output and returns a shorter, structured
version that keeps high-value lines, groups repeated paths, samples large JSON,
and reports how much output was reduced.

## Why Not RTK Hooks Alone

RTK hooks help when a known command shape is already supported, but they are hard
to rely on as the only output-control layer.

- Each tool needs its own hook behavior, so the setup grows like an MCP-style
  adapter list.
- Complex or uncommon command options are limited by what the hook understands.
- Hooks may not run on every path, especially when commands are wrapped,
  aliased, or invoked through another tool.
- Agents can easily bypass the hook by changing the command form.
- Output reduction belongs after the command output exists, not inside every
  individual command adapter.

This tool is meant to work more like the local Unity CLI approach: keep the
command flexible, then pass the resulting output through one reducer that can
recognize common shapes and shrink them consistently.

## Usage

```powershell
rg -n --glob '*.cs' 'ContentAvailability' Assets/Accelix/Scripts | sieve
sieve --mode json --max-chars 8000 < result.json
sieve --mode unity < Editor.log
sieve --tool git < status.txt
sieve --max-input-bytes 4194304 < very-large.log
sieve run -- rg -n --glob '*.cs' 'Foo' Assets
```

The repository name is `token-sieve`; the executable name is `sieve`.

## MVP Modes

- `auto`: detect the input shape.
- `text`: collapse duplicate lines and keep error-like lines first.
- `grep`: group `path:line:text` matches by file.
- `paths`: group file paths by directory.
- `json`: keep object structure and sample long arrays.
- `unity`: keep Unity error, exception, shader, and build lines first.
- `scm`: keep changed, added, deleted, moved, branch, and changeset lines first.

Long repeated path prefixes are shortened automatically:

```text
$root=C:/WorkSpace/ProjectMaid/Assets/Accelix/Scripts
$root/Demo/DemoScopeService.cs (3)
  42: ...
```

SCM output is split into useful sections instead of plain keyword sampling, so
PlasticSCM checkout-only noise and real content changes are easier to separate:

```text
## content change
CO+CH Assets/Changed.asset
CH Assets/Edited.cs
## checkout only
CO Assets/Locked.asset
```

## Tool Hints

```powershell
sieve --tool rg < search.txt
sieve --tool git < status.txt
sieve --tool unity-cli < console.txt
sieve --tool cm --focus DemoScope < status.txt
```

`--tool` is a hint, not a hard parser. It maps common tools to the nearest
shape reducer, so new tools can still work through `auto` detection.

`--focus` keeps comma-separated project terms before generic samples. This is
useful when the important line is not an error, such as `DemoScope`, `Addressables`,
`ProjectBuildConfig`, or a specific content id.

## Limits

```powershell
sieve --max-lines 120 --max-chars 10000 < output.txt
```

Raw input is bounded before reduction:

```powershell
sieve --max-input-bytes 4194304 < output.txt
```

If reduction fails, `--raw-on-fail` is enabled by default so the original output
is not lost.

`sieve run -- <command>` returns the child command exit code after reducing the
combined stdout/stderr output.
