# Human Eye Filter design

`Human Eye Filter` is not a command rewriter. It is an output reducer.

The long-term target is to accept output from many common developer tools and
return a smaller representation without losing the lines that matter for
debugging or decision making.

## Strategy

Do not build reducers around tools. The pipeline looks only at repeated patterns
in the output text and applies a filter only when the result is shorter.

Current minimal filters:

- `RepeatedLine`: collapse consecutive identical lines.
- `SequentialRange`: collapse consecutive numeric runs such as `001..020`.
- `CommonPrefix`: alias long prefixes shared by many lines.
- `DictionaryToken`: alias repeated long paths, URLs, IDs, and GUIDs.
- `BoundedSample`: keep head, important lines, and tail when output is still too large.

Add new filters only when they describe an output pattern, not a command or tool.
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
