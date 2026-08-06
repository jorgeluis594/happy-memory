# Recall Auto-Initialization Design

## Goal

Allow the `happy-memory-recall` skill to recover when a repository has not yet
been initialized, without changing the behavior of the maintainer skill or the
CLI itself.

## Behavior

The recall skill first runs the requested `happy-memory search` command as it
does today. If that command fails with `PROJECT_NOT_INITIALIZED`, the skill runs
`happy-memory init` once and, only when initialization succeeds, repeats the
original search with exactly the same query and filters.

Initialization is attempted at most once during a retrieval. The initialization
command does not count toward the limit of three search attempts. The repeated
search is the completion of the original attempt rather than a newly selected
query.

## Error Handling

- If `happy-memory init` fails, recall returns `failed` with the initialization
  error and does not repeat the search.
- If the repeated search fails, recall returns `failed` with that search error.
- `GIT_REPOSITORY_NOT_FOUND`, `STORE_BUSY`, `STORE_ERROR`, a missing executable,
  and other errors keep their existing behavior and never trigger initialization.
- Recall never initializes proactively; it does so only after observing the
  stable `PROJECT_NOT_INITIALIZED` error.

## Scope

Only these recall instructions and their read contract change:

- `.agents/skills/happy-memory-recall/SKILL.md`
- `.agents/skills/happy-memory-recall/references/search-cli.md`

The `happy-memory-maintainer` skill remains unchanged. No CLI implementation,
storage behavior, or public command contract changes.

## Verification

Review the updated instructions to confirm that they consistently describe:

1. the initial failed search;
2. one conditional `happy-memory init`;
3. one exact retry after successful initialization;
4. preservation of all other failure behavior and search limits.
