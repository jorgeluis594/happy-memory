# happy-memory Maintenance Contract

Use this contract before running any read or mutation command. Run every command from the Git repository associated with the target project.

## Contents

- [Process contract](#process-contract)
- [Memory types](#memory-types)
- [Search related memories](#search-related-memories)
- [Get current state](#get-current-state)
- [Run mutations in a batch](#run-mutations-in-a-batch)
- [Batch response](#batch-response)
- [Search tag vocabulary](#search-tag-vocabulary)
- [Handle stable errors](#handle-stable-errors)

## Process Contract

Treat a command as successful only when it exits with code zero and writes one JSON object containing `ok: true` to stdout. Treat a nonzero exit and JSON containing `ok: false` on stderr as failure. Preserve the stable error code, message, and details.

Do not initialize, repair, or select another project in response to a maintenance failure.

## Memory Types

Choose exactly one type by the nature of the stored idea:

| Type | Meaning |
| --- | --- |
| `fact` | Verifiable current project fact |
| `decision` | Choice and its context or rationale |
| `constraint` | Mandatory rule or limit |
| `preference` | Desired but non-mandatory convention |
| `procedure` | Reusable sequence of steps |
| `lesson` | Known problem, finding, or learned lesson |

Represent topics with tags rather than inventing types.

## Search Related Memories

Run:

```text
happy-memory search "<query>" [filters] --limit 10
```

Use one non-empty, specific textual query. Search only matches active title and content, joins whitespace-delimited terms with AND, and does not interpret synonyms. Apply type, repeated tag, specific tags, minimum importance, or minimum confidence filters only when justified by the maintenance case.

Search results identify candidates but omit their current version. Run `get` before update or delete.

## Get Current State

Run:

```text
happy-memory get <memory-id>
```

Use the returned current version as `expected-version`. Do not mutate from a stale search result.

## Run Mutations in a Batch

Run:

```text
happy-memory batch --input -
```

Use this as the only mutation command, including when there is exactly one operation. Send one strict JSON object through stdin with an `operations` array containing 1 to 100 ordered items:

```json
{"operations":[...]}
```

Every item must be exactly one of the following shapes. Do not target the same existing memory more than once in one batch.

For creation:

```json
{"operation":"create","input":{"type":"fact","title":"Atomic title","content":"Complete content","importance":4,"confidence":5,"attributes":null,"tags":["keyword"]}}
```

The create `input` accepts these fields:

| Field | Requirement |
| --- | --- |
| `type` | One valid type |
| `title` | Non-empty atomic title |
| `content` | Non-empty complete content |
| `importance` | Integer `1` through `5` |
| `confidence` | Integer `1` through `5` |
| `attributes` | Optional JSON object or null |
| `tags` | Zero or more strings or objects with `name` and optional `description` |
| `agent` | Optional object with non-empty `name` and optional `role` |

Do not supply derived ID, version, hashes, dates, project identity, or worktree provenance.

For update:

```json
{"operation":"update","memory_id":"<memory-id>","expected_version":2,"input":{"title":"Revised title"}}
```

The update `input` is a strict JSON patch. Omit an unchanged field. An absent member preserves current state. `attributes: null` removes attributes. `tags: []` removes every association. Any present `tags` member replaces the complete tag set. An effective update increments the version; a no-op patch does not.

For deletion:

```json
{"operation":"delete","memory_id":"<memory-id>","expected_version":2}
```

Delete only after qualifying the deletion case and retrieving the current version. Deletion is logical: it removes the memory from normal retrieval while preserving current data and revision history.

Operations run in input order and are independently atomic. The batch is not an all-or-nothing transaction, so a failed item does not roll back successful items. Use later batches for operations whose inputs or safety conditions depend on earlier results.

## Batch Response

A processed batch writes one response with this structure:

```json
{
  "ok": true,
  "data": {
    "summary": {"total": 2, "succeeded": 1, "failed": 1},
    "results": [
      {"index": 0, "operation": "create", "ok": true, "data": {"id": "<memory-id>", "version": 1}},
      {"index": 1, "operation": "delete", "ok": false, "error": {"code": "MEMORY_NOT_FOUND", "message": "memory not found", "details": {}}}
    ]
  }
}
```

Successful items contain the complete resulting memory in `data`; the abbreviated object above highlights the identity and version fields. Require a zero exit code and `ok: true` for the envelope, then inspect every result in input order. Envelope success does not mean that every operation succeeded. Match each result to its prepared operation by `index` and `operation`, preserve item errors, and run dependent work only after the required items report `ok: true`.

An invalid envelope or operation shape fails the command before the batch is processed. Treat that command-level failure separately from errors returned by processed items.

## Search Tag Vocabulary

Run exactly:

```text
happy-memory tags search "<query>" --limit 10
```

Use a non-empty specific query. Never use `tags list`. Treat the native `--limit 10` flag as part of the required CLI contract.

## Handle Stable Errors

- `VERSION_CONFLICT`: retrieve current state, reassess, and retry the intended operation at most once in a later batch.
- `DUPLICATE_MEMORY`: inspect the existing active memory and route to update or no change.
- `VALIDATION_ERROR`: correct one generated batch or operation when this contract identifies the defect; otherwise stop.
- `MEMORY_NOT_FOUND`: stop the target operation without substituting another ID.
- `GIT_REPOSITORY_NOT_FOUND` or `PROJECT_NOT_INITIALIZED`: stop without initialization or repair.
- `STORE_BUSY` or `STORE_ERROR`: preserve the failure and stop dependent operations.
- Missing executable: report that `happy-memory` is unavailable.

Never convert an error into an empty result. A batch envelope can succeed while items fail; preserve every item error and continue only with operations that do not depend on failed items.
