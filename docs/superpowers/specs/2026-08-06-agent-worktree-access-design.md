# Agent worktree access configuration

## Purpose

`happy-memory` stores one SQLite database at
`dirname(git-common-dir)/.happy-memory/memory.db` so linked Git worktrees share
project memory. An agent running from a secondary worktree may not be allowed to
write that shared directory because it is outside the agent's active workspace.

The first version lets callers explicitly name one or more agents during
initialization. `happy-memory` adds the narrow shared `.happy-memory` directory
to each requested agent's persistent external-write configuration.

## Scope

Support these agent identifiers:

- `codex`
- `claude-code`
- `opencode`

This version will not detect installed or active agents or display an
interactive selector. Multiple explicitly named agents are supported in one
invocation.

## CLI contract

Add an optional flag to `init`:

```text
happy-memory init --configure-agent codex,claude-code,opencode
```

The flag accepts a comma-separated list of supported identifiers, trims
whitespace, removes duplicates in first-seen order, and validates the complete
list before initialization. An empty element or unknown value is a
`VALIDATION_ERROR`. Without the flag, `init` preserves its current behavior and
response contract: it initializes project memory but does not inspect or modify
agent configuration.

The command remains non-interactive and continues to emit exactly one JSON
object. This makes it safe for skills, agents, and scripts.

There is no separate retry command. Since initialization is idempotent, callers
retry the same `init --configure-agent ...` invocation.

## Agent-specific changes

### Codex

Update the user's Codex `config.toml`. Under
`[sandbox_workspace_write]`, add the absolute `.happy-memory` directory to
`writable_roots` while preserving all existing entries and unrelated settings.
Do not broaden access to the repository root and do not use
`danger-full-access`.

If the active configuration uses a permission-profile model that cannot be
safely combined with `sandbox_mode` or `[sandbox_workspace_write]`, return an
unsupported or policy-conflict result instead of rewriting the security model.

### Claude Code

Update the user's Claude Code settings. Add the absolute `.happy-memory`
directory to `permissions.additionalDirectories`, preserving existing settings
and entries.

### OpenCode

Update the existing global OpenCode configuration file selected by OpenCode's
documented precedence, or its default global configuration file when none
exists. Add an `allow` rule for `<absolute .happy-memory path>/**` under
`permission.external_directory`, preserving existing rules and unrelated
settings.

## Configuration result

Agent configuration is a separate outcome from project initialization. The
successful `init --configure-agent` response includes an ordered
`agent_configurations` array. Every item contains:

- `agent`
- `status`: `configured`, `already_configured`, or `warning`
- `config_path`
- `shared_memory_path`
- a structured `warning` when configuration could not be applied

Every requested agent is attempted even when an earlier one returns a warning.
Project initialization remains successful if agent configuration is blocked by
a sandbox, invalid file, or managed policy. The response reports a structured
warning inside JSON and must not claim that synchronization is ready.

## Authorization boundary

The CLI cannot invoke an agent product's approval dialog directly. It attempts
only the requested configuration write. If the host sandbox denies that write,
the structured result instructs the calling skill or agent to request scoped
authorization and retry the same idempotent `happy-memory init
--configure-agent <agents>` command.

Agent-specific skills should always pass their own explicit identifier. For
example, the Codex skill invokes `--configure-agent codex`; it must not configure
Claude Code or OpenCode merely because they are installed.

## Safe file updates

Configuration updates must:

- resolve and display absolute target paths;
- preserve unrelated values and existing entries;
- avoid duplicate paths and rules;
- validate the source document before mutation;
- write through a same-directory temporary file and atomic rename;
- preserve the original file permissions when replacing an existing file;
- use user-only permissions for a newly created user configuration file;
- leave the original untouched if parsing, validation, or replacement fails.

No configuration file is modified when the requested access already exists.

Comment preservation is required for TOML and JSONC formats that support
comments. If the selected parser cannot round-trip a file without losing its
comments or structure, the command returns `unsupported` rather than replacing
it destructively.

## Components

Keep agent configuration separate from project initialization:

1. The Cobra adapter validates and normalizes the comma-separated agent list.
2. An application service resolves the shared memory directory and coordinates
   one agent configurator.
3. Each agent configurator owns config discovery, compatibility checks,
   idempotence, and document mutation for one format.
4. A filesystem writer owns permission preservation and atomic replacement.

This boundary allows future interactive selection or agent detection to call
the same per-agent configurators without coupling prompt behavior to
configuration formats.

## Testing

Cover:

- `init` without the flag remains byte-for-byte compatible where its existing
  response is asserted;
- all supported and unsupported flag values;
- multiple agents, whitespace normalization, deduplication, and stable order;
- missing, existing, and already-configured files;
- preservation of unrelated settings, comments, ordering where required, and
  file permissions;
- malformed TOML, JSON, and JSONC;
- Codex permission-profile conflicts and managed configuration;
- paths containing spaces and platform path separators;
- idempotent repeated configuration;
- atomic-write failures leave the source unchanged;
- sandbox denial produces an actionable structured result;
- linked worktrees resolve the same shared `.happy-memory` directory;

Integration fixtures should use temporary home/config directories and must not
modify the developer's real agent configuration.

## Documentation

Document the shared-worktree problem, the three supported agent identifiers,
the exact commands, the narrow permissions granted, and the fact that a calling
agent may need scoped approval to update its global configuration.
