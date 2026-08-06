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
- Agent and worktree provenance for every revision.
- One memory database shared by every linked worktree of a repository.
- Stable JSON success and error envelopes, plus non-destructive storage
  diagnostics.

Semantic interpretation remains outside the CLI. `happy-memory` does not use
embeddings, infer search intent, or decide which memories an agent should keep.

## Installation

> **Release status:** the commands below will work after the first GitHub
> Release is published. The release pipeline is currently ready for snapshot
> validation, but no public version has been tagged yet.

On Linux or macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/jorgeluis594/happy-memory/main/install.sh | sh
```

On Windows with Windows PowerShell 5.1 or PowerShell 7:

```powershell
irm https://raw.githubusercontent.com/jorgeluis594/happy-memory/main/install.ps1 | iex
```

The installers select the latest stable release, verify its SHA-256 checksum,
validate the executable, and preserve an existing installation if an update
fails. They do not use `sudo` or require administrator privileges.

To download and review the script before running it:

```sh
curl -fsSLO https://raw.githubusercontent.com/jorgeluis594/happy-memory/main/install.sh
less install.sh
sh install.sh
```

```powershell
irm https://raw.githubusercontent.com/jorgeluis594/happy-memory/main/install.ps1 -OutFile install.ps1
Get-Content .\install.ps1
.\install.ps1
```

Install an explicit version or choose another directory:

```sh
sh install.sh --version v0.1.0 --bin-dir "$HOME/bin"
```

```powershell
.\install.ps1 -Version v0.1.0 -BinDir "$HOME\bin"
```

Unix installations default to `~/.local/bin`; add it to `PATH` if the installer
prints that guidance. Windows installations default to
`%LOCALAPPDATA%\Programs\happy-memory\bin`, which the installer adds once to the
user-level `PATH`.

### Supported binaries and manual installation

Releases provide standalone binaries for Linux, macOS, and Windows on `amd64`
and `arm64`. Download the archive matching your platform from GitHub Releases,
download `checksums.txt` from the same release, verify the archive's SHA-256,
and extract its sole `happy-memory` or `happy-memory.exe` file into a directory
on `PATH`. No Go or SQLite installation is required.

Confirm any installation with:

```sh
happy-memory version
```

To uninstall on Unix, remove `~/.local/bin/happy-memory` (or the custom target).
On Windows, remove `happy-memory.exe`, remove its bin directory from the user
`PATH` if desired, and delete an empty installation directory. Uninstallation
does not remove repository `.happy-memory` data.

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

Coding-agent sandboxes may treat that shared directory as external when they
run from a linked worktree. Initialize with one or more explicit agent names to
add the narrow `.happy-memory` directory to their persistent configuration:

```text
happy-memory init --configure-agent codex
happy-memory init --configure-agent codex,claude-code,opencode
```

Supported names are `codex`, `claude-code`, and `opencode`. The flag is
idempotent, trims whitespace, and ignores duplicate names. Agent configuration
failures are returned as structured warnings without failing a successful
project initialization, so the same `init` command can be retried with the
required scoped authorization. Omitting the flag preserves the original
initialization behavior and JSON response.

## Development

### Requirements

- Go 1.26.1 or later
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

Release work can be checked locally with:

```bash
make release-check
make release-snapshot
make release-validate
make installers-test
```

These developer targets require GoReleaser v2 and standard archive/checksum
utilities. GitHub Actions runs this release workflow only for pushed tags. Tags
matching `vMAJOR.MINOR.PATCH` run native smoke tests and `make check`, then
publish only when no release already exists for that tag. Other `v*` tags are
rejected before publication.

## Documentation

- [Product specification](docs/memory-cli-product-design.md)
- [Architecture guidelines](docs/architecture.md)
