# happy-memory Read Contract

## Contents

- [Search command](#search-command)
- [Filter contract](#filter-contract)
- [Text and scope semantics](#text-and-scope-semantics)
- [Ranking](#ranking)
- [Search response](#search-response)
- [Tag vocabulary](#tag-vocabulary)
- [Process and error contract](#process-and-error-contract)
- [Examples](#examples)

## Search command

Run exactly one positional query:

```text
happy-memory search "<query>" [filters]
```

Quote a multiword query at the shell boundary so Cobra receives it as one argument. The query must remain non-empty after trimming whitespace.

The command defaults to `--limit 10`. It exposes no interactive mode, semantic mode, profile, configurable ranking weights, or queryless bootstrap search.

Run 1–100 already-selected independent searches in one ordered request:

```text
happy-memory search --input -
{"searches":[{"query":"sqlite"},{"query":"worktrees","type":"decision","tags":["git"],"limit":5}]}
```

Batch entries accept `query`, `type`, `tags`, `specific_tags`, `min_importance`,
`min_confidence`, and `limit`. An omitted `limit` defaults to `10`. Do not mix
`--input` with a positional query or search filter flags.

## Filter contract

| Flag | Accepted value | Effect |
| --- | --- | --- |
| `--type` | One exact type | Retain only that memory type |
| `--tag` | Non-empty tag; repeatable | Retain memories containing every supplied tag |
| `--min-importance` | Integer `1` through `5` | Retain `importance >= value` |
| `--min-confidence` | Integer `1` through `5` | Retain `confidence >= value` |
| `--limit` | Integer `1` through `100` | Return at most that many ranked results |

Valid types are:

| Type | Stored meaning |
| --- | --- |
| `fact` | Verifiable current fact |
| `decision` | Choice and its context or rationale |
| `constraint` | Mandatory rule or limit |
| `preference` | Desired but non-mandatory convention |
| `procedure` | Reusable sequence of steps |
| `lesson` | Known problem, finding, or learned lesson |

Omit an unused flag. Do not pass zero for a minimum even though zero represents omission inside the application.

Tag handling:

- Normalize lookup values by lowercasing, trimming outer whitespace, replacing whitespace runs with `-`, and collapsing repeated hyphens.
- Treat repeated tags as AND.
- Deduplicate repeated normalized tag values in `search`.
- Reject an empty normalized tag.

The CLI has no search filters for date, version, memory ID, branch, worktree, attributes, author, maximum importance, maximum confidence, OR, NOT, excluded tags, multiple types, or deleted memories. Express OR only as separate search commands chosen by the primary agent.

## Text and scope semantics

- Resolve the project from the current Git repository. No search flag can select another project.
- Search only current, active memories in that project.
- Never return a soft-deleted memory.
- Match only the current `title` and `content` fields.
- Do not match tags, attributes, revisions, history, or deleted content.
- Split the trimmed query on whitespace.
- Escape each term as literal FTS5 text and join all terms with AND.
- Return an empty successful result without querying storage when the query contains punctuation only and no letter or number.

The CLI performs deterministic textual search. It does not call a model, generate embeddings, expand synonyms, or interpret semantic intent.

## Ranking

The repository applies structured filters before selecting a fixed window of at most 100 BM25 candidates. BM25 weights title by `5` and content by `1`.

Ranking version 1 computes:

```text
importance_score = (importance - 1) / 4
confidence_score = (confidence - 1) / 4
final_score = 0.70 * text_score + 0.20 * importance_score + 0.10 * confidence_score
```

Normalize `text_score` by min-max over the candidate window. Assign `text_score = 1` to every candidate when the window contains one candidate or all BM25 values are equal.

Apply tie-breakers in this order:

1. `final_score DESC`
2. `importance DESC`
3. `confidence DESC`
4. `updated_at DESC`
5. `memory_id ASC`

Apply the requested `--limit` after ranking the candidate window. Compare scores only inside the same search response; separate searches normalize against separate candidate windows.

## Search response

Successful search writes JSON to stdout and exits with code zero:

```json
{
  "ok": true,
  "data": {
    "ranking_version": 1,
    "results": [
      {
        "id": "memory-id",
        "type": "decision",
        "title": "Use Git worktrees",
        "content": "Linked worktrees share project memory.",
        "importance": 5,
        "confidence": 4,
        "tags": ["git", "worktrees"],
        "score": {
          "final": 0.9,
          "text": 1,
          "importance": 1,
          "confidence": 0.75
        }
      }
    ]
  }
}
```

An empty match set is:

```json
{"ok":true,"data":{"ranking_version":1,"results":[]}}
```

Search results do not expose memory version, attributes, content hash, timestamps, deletion timestamp, tag IDs, or project ID.

A batch returns `ok: true`, a `total`/`succeeded`/`failed` summary, and one
ordered item per input with `index`, `ok`, and either the normal search `data`
or a public `error`. Item failures do not stop later searches or produce a
nonzero exit. Malformed JSON, unknown fields, trailing content, a batch outside
1–100, and failures opening storage or resolving the project fail globally.

## Tag vocabulary

List the complete current-project vocabulary:

```text
happy-memory tags list
```

Search tag name and description by a non-empty, literal, case-insensitive substring:

```text
happy-memory tags search "<query>"
```

`tags list` orders by normalized name ascending. `tags search` orders by active-memory count descending and then normalized name ascending. Both commands include tags with zero active memories.

Both commands return:

```json
{
  "ok": true,
  "data": {
    "tags": [
      {
        "id": "tag-id",
        "name": "SQLite",
        "normalized_name": "sqlite",
        "description": "SQLite persistence",
        "created_at": "2026-08-05T12:00:00Z",
        "updated_at": "2026-08-05T12:00:00Z",
        "active_memory_count": 3
      }
    ]
  }
}
```

Tag discovery does not select or apply a tag. Return the vocabulary to the primary agent for that decision.

## Process and error contract

Every successful command:

- Writes one JSON object to stdout.
- Exits with code zero.
- Uses `ok: true`.

Every failed command:

- Writes one JSON object to stderr.
- Exits with a nonzero code.
- Uses this shape:

```json
{
  "ok": false,
  "error": {
    "code": "PROJECT_NOT_INITIALIZED",
    "message": "project is not initialized",
    "details": {}
  }
}
```

Read operations may surface these stable codes:

| Code | Meaning for retrieval |
| --- | --- |
| `GIT_REPOSITORY_NOT_FOUND` | Current directory is not inside a Git repository |
| `PROJECT_NOT_INITIALIZED` | Repository has no usable happy-memory identity |
| `VALIDATION_ERROR` | Query, filter, argument count, or command is invalid |
| `STORE_BUSY` | Storage remained busy after bounded retries |
| `STORE_ERROR` | Storage, schema, migration, or internal read failed |

Treat a missing `happy-memory` executable as a local execution failure rather than a CLI JSON error.

When a search fails with `PROJECT_NOT_INITIALIZED`, run `happy-memory init --configure-agent <current-agent>` once, selecting exactly one of `codex`, `claude-code`, or `opencode` for the agent executing the skill. Request scoped authorization when the host requires it to update global agent configuration. If initialization succeeds, repeat the original search with exactly the same query and filters. Preserve any `agent_configurations` warning and do not claim shared-worktree synchronization is configured. The initialization command does not count toward the three-search limit, and the repeated command completes the original search attempt. If initialization fails, preserve and return its error without repeating the search. If the repeated search fails, preserve and return that search error.

Do not initialize for any other read failure. Never use any other mutation command as recovery.

## Examples

Search with the default limit:

```sh
happy-memory search "sqlite retry"
```

Search with every supported filter:

```sh
happy-memory search "schema migration" --type decision --tag sqlite --tag migrations --min-importance 4 --min-confidence 3 --limit 20
```

Run separate alternatives selected by the primary agent:

```sh
happy-memory search "retry" --limit 10
happy-memory search "contention" --limit 10
```

Inspect tag vocabulary:

```sh
happy-memory tags search "database"
happy-memory tags list
```
