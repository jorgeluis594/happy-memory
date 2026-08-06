# happy-memory

`happy-memory` is a local, deterministic memory store for AI agents working in
Git repositories. It gives agents a durable way to preserve project facts,
decisions, constraints, preferences, procedures, and lessons without treating
conversation history as a database.

The project is implemented as a Go CLI with stable JSON contracts and a
repository-scoped SQLite store. Agents decide what knowledge is useful; the CLI
validates, persists, versions, and retrieves it.

## What it provides

- Atomic, typed memories with required importance and confidence scores.
- Repository identity and strict project-level data isolation.
- Full revision history with optimistic concurrency control.
- Soft deletion and restoration without losing prior states.
- Project-scoped tags with a reusable canonical vocabulary.
- Deterministic FTS5 search ranked by text relevance, importance, and
  confidence.
- Independently atomic batch mutations for agent maintenance workflows.
- Specialized subagent provenance, including agent name, functional role, and
  originating worktree for every revision.
- One memory database shared by every linked worktree of a repository.
- Stable JSON success and error envelopes, plus non-destructive storage
  diagnostics.

The `agent.role` field accepts free-form, non-empty values such as `qa`,
`copywriter`, `reviewer`, or `database-specialist`. It records the function a
specialized subagent performed; it does not grant permissions or affect search
ranking. When omitted, the stored role is `unknown`.

Semantic interpretation remains outside the CLI. `happy-memory` does not use
embeddings, infer search intent, or decide which memories an agent should keep.

## Agent integration

Agents normally interact with the store through repository skills rather than
through a human-oriented command workflow:

- [`happy-memory-recall`](.agents/skills/happy-memory-recall/SKILL.md) performs
  read-only, deterministic retrieval.
- [`happy-memory-maintainer`](.agents/skills/happy-memory-maintainer/SKILL.md)
  creates, updates, splits, soft-deletes, and curates durable project knowledge.

These skills keep semantic judgment in the agent while treating the CLI as the
source of truth for validation, concurrency, persistence, and history.

## Storage and repository identity

Initialization assigns the Git repository a UUID under
`happy-memory.project-id` in its common local Git configuration. Linked
worktrees share that identity, while an independent clone receives a different
one.

The SQLite database is stored at:

```text
dirname(git-common-dir)/.happy-memory/memory.db
```

This location makes the database available to all linked worktrees while each
operation remains isolated by project ID. The storage directory and database
are created with permissions restricted to the current user. Normal commands
open existing storage and do not initialize a project implicitly.

## Development

### Requirements

- Go 1.26
- Git
- Make

### Quality checks

```bash
make build
make test
make check
```

`make test` runs the test suite with the race detector and coverage. `make
check` additionally verifies formatting, linting, known vulnerabilities,
module checksums, and the CLI build.

The Makefile also exposes focused targets through `make help`.

## Documentation

- [Product specification](docs/memory-cli-product-design.md)
- [Architecture guidelines](docs/architecture.md)
- [POC implementation plan](docs/poc/README.md)
