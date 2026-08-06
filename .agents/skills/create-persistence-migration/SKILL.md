---
name: create-persistence-migration
description: "Create and validate a versioned SQLite migration with Goose in happy-memory from a complete migration specification supplied by the calling agent. Use when an agent requests a concrete migration with the required Up schema change, Down behavior, data transformations, constraints, and indexes already defined."
metadata:
  internal: true
---

# Create a Persistence Migration

Turn the calling agent's complete specification directly into a versioned Goose SQL migration. Treat the supplied specification as authoritative; do not rediscover product requirements or expand the task into persistence-layer implementation.

## Input Contract

Expect the calling agent to provide:

- The migration name or purpose.
- The schema change required in `Up`.
- The required `Down` behavior, or an explicit statement that no safe rollback exists.
- Any required data transformations.
- Relevant constraints, foreign keys, indexes, defaults, and nullability rules.

Ask only for missing or contradictory information that prevents safe SQL generation. Do not inspect POC tasks, models, repositories, or queries to infer requirements.

## Workflow

### 1. Resolve the Migration Target

- Use `rg` to locate the migration directory, its `embed.FS`, and `sqlite.Migrate`.
- Inspect existing migration filenames and choose the next available five-digit sequential version.
- Derive a short snake-case filename from the supplied name, for example `00002_add_project_name_index.sql`.
- Review `git status --short` and preserve unrelated changes.

If this is the first migration and no embedded migration source exists, create the minimum `embed.FS` integration required to expose the migration files to `sqlite.Migrate`. Do not implement models, repositories, queries, or other persistence behavior.

### 2. Create the Migration First

Create the SQL file immediately after resolving its path and version. Encode only the supplied specification.

```sql
-- +goose Up
-- supplied forward change

-- +goose Down
-- supplied safe rollback
```

- Use valid SQLite SQL compatible with the project's pure-Go driver.
- Use Goose's default transaction.
- Add `-- +goose NO TRANSACTION` only when technically required.
- Use `StatementBegin` and `StatementEnd` only when Goose must parse multiple statements as one block.
- Include `Down` exactly as specified. If the caller explicitly marks the migration as irreversible, do not invent a destructive rollback; omit `Down` and report the limitation.
- Consult the current Pressly Goose documentation through Context7 before relying on annotation or API details, following `AGENTS.md`.

### 3. Validate the Artifact

Validate only the migration and its immediate Goose integration:

- Confirm that the filename is sequential and unique.
- Confirm that Goose annotations are present and ordered correctly.
- Confirm that the `embed.FS` includes the new file.
- Apply the migration to a temporary SQLite database through `sqlite.Migrate` when the repository's current infrastructure supports it.
- Run the existing focused migration test package when available.
- Run `git diff --check` for the modified files.

Do not add tests outside the migration artifact's immediate Goose integration. Those remain the calling agent's responsibility.

### 4. Hand Off

Report:

- The migration path and version.
- Whether `Down` is available.
- Any minimal `embed.FS` integration created for the first migration.
- The focused validation commands and their results.

Do not update GORM models, repositories, queries, DTOs, POC documentation, or unrelated application code.

## Guardrails

- Do not use `AutoMigrate`.
- Do not renumber, delete, or rewrite historical migrations that may have been applied.
- Do not truncate or replace the user's database.
- Do not invent requirements beyond the supplied migration specification.
- Do not implement unapproved data loss.
