---
name: happy-memory-recall
description: "Retrieve read-only repository memory through the happy-memory CLI. Use automatically before designing, implementing, modifying, diagnosing, or making decisions about a repository, and whenever a user or agent explicitly asks to recall memory, recover prior context, consult previous decisions, or search stored memory. Do not invoke automatically for status-only inspection, formatting-only work, or execution of an already specified command or test."
---

# Recall Memory with happy-memory

Retrieve memory candidates without taking ownership of the primary agent's judgment.

## Keep Decisions with the Primary Agent

As the primary agent:

- Choose every search query and filter.
- Evaluate the returned candidates.
- Decide whether to stop or request another search.
- Decide how to use any retrieved memory in the task.

Do not add query-selection heuristics, infer filters, invent synonyms, or label results as relevant, sufficient, or noisy as part of this skill's procedure.

## Prepare the Retrieval

1. Read [references/search-cli.md](references/search-cli.md) before the first CLI call of each retrieval.
2. Run the CLI from the repository associated with the task.
3. Supply a non-empty query and optionally supply one type, zero or more tags, minimum importance, minimum confidence, and a limit.
4. Use a limit of `10` when the primary agent does not choose one.
5. Validate the selected values against the reference before execution.

## Execute One Attempt

1. Run `happy-memory search <query>` with exactly the selected filters. When two
   or three independent searches have already been selected, group them in one
   ordered `happy-memory search --input -` request instead. Never add a search
   merely to fill a batch.
2. If the command fails with `PROJECT_NOT_INITIALIZED`, run `happy-memory init --configure-agent <current-agent>` once, using `codex`, `claude-code`, or `opencode` for the agent executing the skill. Configure only the current agent. When the host requires scoped authorization to update global agent configuration, request it for this command. Treat `agent_configurations[].status: warning` as successful project initialization but report that shared-worktree access is not configured; do not silently claim synchronization is ready. When initialization succeeds, repeat the original search with exactly the same query and filters. Treat the repeated search as the completion of the same attempt, and do not count `init` as a search command.
3. Treat stdout as success only when the process exits with code zero and the JSON contains `ok: true`.
4. Treat stderr as failure when the process exits with a nonzero code.
5. Preserve each result's title and content verbatim.
6. Record the query, filters, `ranking_version`, result count, result order, and score components.
7. Return control to the primary agent after recording the attempt.

Execute another attempt only when the primary agent chooses a new query or filter set. Execute at most three search attempts in one retrieval, counting each batch entry as one attempt. Count a corrected entry after `VALIDATION_ERROR` toward this maximum.

## Look Up Tags

Run `happy-memory tags search <query>` or `happy-memory tags list` only when the primary agent requests vocabulary lookup. Record all returned tag fields and return them without selecting a tag. Tag lookup commands do not count toward the three-search maximum.

## Accumulate Results

- Deduplicate memories by ID across attempts.
- Keep the first returned copy of a duplicated memory.
- Add one `matches` entry for every attempt that returned the memory.
- Preserve rank and score within the originating attempt.
- Never compare, merge, average, or globally sort scores from different attempts.
- Preserve first-discovery order across attempts.
- For a partial batch failure, preserve and accumulate every successful item in
  its original position and record every failed item's public error. Do not
  retry or discard successful siblings.

Build this YAML packet in working context:

```yaml
status: found | empty | failed
attempts:
  - query: "sqlite retry"
    filters:
      type: lesson
      tags: []
      min_importance: null
      min_confidence: null
      limit: 10
    ranking_version: 1
    result_count: 1
tag_lookups:
  - command: search
    query: "sqlite"
    tags:
      - id: "tag-id"
        name: "SQLite"
        normalized_name: "sqlite"
        description: "SQLite persistence"
        created_at: "2026-08-05T12:00:00Z"
        updated_at: "2026-08-05T12:00:00Z"
        active_memory_count: 3
memories:
  - id: "memory-id"
    type: lesson
    title: "Retry SQLite writes"
    content: "Preserve this content verbatim."
    tags: ["sqlite"]
    importance: 4
    confidence: 5
    matches:
      - attempt: 1
        rank: 1
        score:
          final: 0.95
          text: 1
          importance: 0.75
          confidence: 1
error: null
```

Use `command: search` with the supplied query for `tags search`. Use `command: list` and `query: null` for `tags list`. Use empty arrays for `attempts`, `tag_lookups`, or `memories` when the retrieval has no entries of that kind.

Set the final status by observable outcome:

- Use `failed` when an error prevents completion. Failure takes precedence over prior successful attempts.
- Otherwise, use `found` when at least one successful attempt returned a memory. This status does not assert relevance.
- Otherwise, use `empty` when every successful attempt returned `results: []`.

For `failed`, preserve any candidates from earlier successful attempts and populate `error.code`, `error.message`, and `error.details` from the CLI response. If the executable is missing, use a null code, the message `happy-memory executable not found`, and empty details.

## Handle Failures

- If `happy-memory` is unavailable, return `failed` and report that the executable is missing.
- On `PROJECT_NOT_INITIALIZED`, initialize only through the one-time recovery in **Execute One Attempt**, passing only the current agent identifier. If initialization fails, return `failed` with the initialization error. If initialization succeeds with an agent-configuration warning, preserve that warning in the failure packet while continuing the repeated search. If the repeated search fails, return `failed` with that search error.
- On `GIT_REPOSITORY_NOT_FOUND`, `STORE_BUSY`, or `STORE_ERROR`, return `failed` without initializing, mutating, or repairing anything.
- On `VALIDATION_ERROR` from a generated command, correct the invocation once using the reference. Return `failed` if the corrected invocation fails.
- Never convert a CLI error into `empty`.
- Except for the one-time `init` recovery, never invoke `create`, `update`, `delete`, `restore`, or any other mutating operation.
