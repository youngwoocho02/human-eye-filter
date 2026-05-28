# token-sieve design

`token-sieve` is not a command rewriter. It is an output reducer.

The long-term target is to accept output from many common developer tools and
return a smaller representation without losing the lines that matter for
debugging or decision making.

## Strategy

Do not build hundreds of bespoke parsers first. Most tool output falls into a
small set of shapes:

- `path:line:text` search matches
- path lists
- JSON objects and arrays
- status tables
- build, compiler, test, and runtime logs
- diff-like or SCM summaries
- stack traces

The first reducer layer handles these shapes. Tool-specific support is a hint
that selects a reducer and tweaks priorities.

Examples:

- `rg`, `grep`, `ag`, `ack`: grep reducer
- `find`, `fd`: path reducer
- `git`, `gh`, `cm`, `svn`, `hg`, `jj`: SCM reducer
- `unity-cli`, `unity-scanner`, `dotnet`, `go`, `npm`, `cargo`: log reducer
- `curl`, `jq`, `gh api`: JSON reducer when the payload is JSON

## Future local agent layer

A local agent can be added later, but it should sit behind the reducer. The
cheap deterministic reducers should run first. A local agent should only see
already-short output or a bounded raw excerpt.

Good use cases for a local agent:

- naming repeated error families
- selecting which raw section should be opened next
- mapping tool-specific noise to known causes

Bad use cases for a local agent:

- reading unbounded logs
- replacing deterministic grouping
- making command execution decisions in MVP

## Raw retention

The MVP does not maintain a raw output database. If raw retention is needed,
add explicit `--save-raw <path>` later rather than hidden caching.
