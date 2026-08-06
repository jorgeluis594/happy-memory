# Batch-only memory maintainer skill design

## Context

The `happy-memory` CLI supports ordered batches of 1 to 100 `create`, `update`, and `delete` operations through `happy-memory batch --input -`. Each operation is independently atomic, the command processes every valid envelope item in order, and the response reports per-item success or failure. The batch itself is not an all-or-nothing transaction.

The current `happy-memory-maintainer` skill predates that command. It instructs agents to execute individual mutations and states incorrectly that the CLI has no batch operation. Its mutation-specific references also document direct `create`, `update`, and `delete` commands.

## Goal

Make `batch` the only mutation path used by the `happy-memory-maintainer` skill, including when the maintainer needs to execute exactly one mutation. Remove instructions that tell agents to run mutation commands individually while preserving memory atomicity, dependency ordering, optimistic concurrency, and per-operation verification.

## Non-goals

- Change the CLI implementation or its batch semantics.
- Treat a batch as a global transaction.
- Combine independent durable ideas into one memory.
- Replace read commands such as `search`, `get`, or tag vocabulary lookup.
- Add `restore` to maintenance flows; the current batch command supports only `create`, `update`, and `delete`.

## Documentation structure

### Skill entry point

`SKILL.md` will state the invariant that every mutation is submitted with `happy-memory batch --input -`. The maintenance cycle will prepare all currently executable operations, submit them as a batch, verify the command envelope and every ordered item result, and then prepare later batches for operations whose inputs depend on earlier results.

The skill will continue to split knowledge into atomic memories. Batching changes command transport and project-resolution overhead, not the semantic boundary of a memory.

### Central CLI contract

`references/cli-contract.md` will contain the sole executable mutation contract. Its separate direct-command sections for create, update, and delete will be replaced by a batch section that documents:

- the required `happy-memory batch --input -` invocation;
- the strict top-level `operations` array;
- the 1-to-100 operation limit;
- the shapes of `create`, `update`, and `delete` items;
- the prohibition against targeting the same existing memory more than once in a batch;
- ordered execution and ordered result entries;
- independently atomic operations and partial success;
- envelope-level versus item-level failures;
- validation of both the process response and each item result.

Read commands will remain documented separately because they are not mutations.

### Case-specific references

The creation, update/division, and deletion references will explain how to prepare batch operations without repeating the CLI wire contract:

- Creation will collect qualified independent candidates into `create` items and submit all currently independent items together.
- Update will prepare minimal patches as `update` items after retrieving current versions.
- Deletion will prepare `delete` items only after qualifying deletion and retrieving current versions.
- Division will use successive batches when the original update or deletion depends on successful replacement creations.

No case-specific reference will instruct the agent to invoke `happy-memory create`, `happy-memory update`, or `happy-memory delete` directly.

## Execution flow

1. Discover and classify durable candidates.
2. Retrieve related active memories, current versions, and tag vocabulary as required.
3. Prepare all mutation operations whose inputs and safety conditions are already known.
4. Submit those operations in one batch, even when there is only one operation.
5. Require a zero exit status and a response with `ok: true` for the batch envelope.
6. Inspect every result in input order and record each operation as succeeded or failed.
7. Reassess retries and dependent operations from the observed item results.
8. Submit another batch only for operations that are now safe and ready.

## Failure behavior

A valid batch may return `ok: true` while individual operations fail. The maintainer must not treat envelope success as proof that all mutations succeeded. It will preserve stable item error codes and continue only with operations that do not depend on failed items.

`VERSION_CONFLICT` may trigger one fresh `get`, reassessment, and retry in a later batch. `DUPLICATE_MEMORY` may route the candidate to update or no change. Storage, project, validation, missing-executable, and not-found behavior remains conservative. An invalid batch envelope prevents execution of the entire batch and is handled separately from item failures.

## Validation

After editing the skill:

1. Search the maintainer skill directory for direct mutation invocations and statements that require one mutation command per memory.
2. Confirm the only executable mutation command documented by the skill is `happy-memory batch --input -`.
3. Confirm the batch contract describes single-operation batches, partial success, ordered per-item verification, and dependent follow-up batches.
4. Review the final diff for consistency across `SKILL.md` and all mutation-specific references.

