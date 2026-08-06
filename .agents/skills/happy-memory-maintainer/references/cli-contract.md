# happy-memory Maintenance Contract

Use this contract before running any read or mutation command. Run every command from the Git repository associated with the target project.

## Contents

- [Process contract](#process-contract)
- [Memory types](#memory-types)
- [Search related memories](#search-related-memories)
- [Get current state](#get-current-state)
- [Create](#create)
- [Update](#update)
- [Delete](#delete)
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

Use one non-empty, specific textual query. Search only matches active title and content, joins whitespace-delimited terms with AND, and does not interpret synonyms. Apply type, repeated tag, minimum importance, or minimum confidence filters only when justified by the maintenance case.

Search results identify candidates but omit their current version. Run `get` before update or delete.

## Get Current State

Run:

```text
happy-memory get <memory-id>
```

Use the returned current version as `expected-version`. Do not mutate from a stale search result.

## Create

Run:

```text
happy-memory create --input -
```

Send exactly one strict JSON object through stdin with these fields:

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

## Update

Run:

```text
happy-memory update <memory-id> --expected-version <n> --input -
```

Send one strict JSON patch through stdin. Omit an unchanged field. An absent member preserves current state. `attributes: null` removes attributes. `tags: []` removes every association. Any present `tags` member replaces the complete tag set. An effective update increments the version; a no-op patch does not.

## Delete

Run:

```text
happy-memory delete <memory-id> --expected-version <n>
```

Delete only after qualifying the deletion case and retrieving the current version. Deletion is logical: it removes the memory from normal retrieval while preserving current data and revision history.

## Search Tag Vocabulary

Run exactly:

```text
happy-memory tags search "<query>" --limit 10
```

Use a non-empty specific query. Never use `tags list`. Treat the native `--limit 10` flag as part of the required CLI contract.

## Handle Stable Errors

- `VERSION_CONFLICT`: retrieve current state, reassess, and retry the intended mutation at most once.
- `DUPLICATE_MEMORY`: inspect the existing active memory and route to update or no change.
- `VALIDATION_ERROR`: correct one generated invocation when this contract identifies the defect; otherwise stop.
- `MEMORY_NOT_FOUND`: stop the target mutation without substituting another ID.
- `GIT_REPOSITORY_NOT_FOUND` or `PROJECT_NOT_INITIALIZED`: stop without initialization or repair.
- `STORE_BUSY` or `STORE_ERROR`: preserve the failure and stop dependent mutations.
- Missing executable: report that `happy-memory` is unavailable.

Never convert an error into an empty result. When several independent memories are being maintained, continue only with operations that do not depend on the failed command.
