# Update and Divide Memories

Update when an active memory still represents the same durable idea but its current state needs correction, refinement, rescoring, retagging, or division.

## Preserve Identity

Treat the memory as the same idea when its durable subject and purpose remain continuous despite changes in wording or current state. Create a separate memory instead when the new information can evolve independently.

Never append an independent idea merely because a related memory already exists.

## Update One Memory

1. Retrieve the current memory with `get` and record its version.
2. Compare every proposed field with the current state.
3. Re-evaluate content, type, importance, confidence, and tags only where evidence or impact changed.
4. Send a minimal patch containing only changed fields.
5. When including `tags`, send the complete tag set that must remain after the update.
6. Run `update` with the retrieved version as `expected-version`.
7. Verify the exit code and `ok: true` response.
8. Treat an identical resulting state as no change.

When preserving obsolete knowledge is itself durable, rewrite the memory so its current content explicitly states the obsolescence. Score that current assertion rather than the superseded state.

## Divide a Memory

Treat division as a coordinated update:

1. Enumerate the independent durable ideas in the current memory.
2. Decide whether the original memory can coherently represent one resulting idea.
3. Prepare each additional idea as a separate creation with its own type, scores, and tags.
4. Create and verify all required new memories first.
5. Update the original to one resulting idea when possible.
6. Verify that every durable part of the original is preserved.
7. Delete the original only when it cannot remain as an atomic result and all replacements succeeded.

Stop the dependent division sequence when a replacement fails. Do not leave the original missing information.

## Resolve Concurrency and Duplicates

- On `VERSION_CONFLICT`, retrieve the new current state, reassess the proposed change, and retry once only if it remains correct.
- On `DUPLICATE_MEMORY`, inspect the identified active memory and route to update or no change.
- On an uncorrected validation or storage error, preserve the failure and stop dependent mutations.
