# token-sieve

Token-aware output reducer for agents.

`token-sieve` accepts arbitrary command output and returns a shorter, structured
version that keeps high-value lines, groups repeated paths, samples large JSON,
and reports how much output was reduced.

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

## Tool Hints

```powershell
sieve --tool rg < search.txt
sieve --tool git < status.txt
sieve --tool unity-cli < console.txt
```

`--tool` is a hint, not a hard parser. It maps common tools to the nearest
shape reducer, so new tools can still work through `auto` detection.

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
