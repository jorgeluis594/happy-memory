# Isolated Memory Recall Implementation Plan

## Objective

Implement isolated memory retrieval entirely through the existing batch search,
exact canonical tag filters, and tokenized `specific_tags`. Recall delegates raw
candidates to one non-recursive subagent, and maintainer improves the selection
of general and specific tags. No CLI, storage, migration, or ranking change is
required.

Design source:
`docs/superpowers/specs/2026-08-13-isolated-memory-recall-design.md`.

## Constraints

- Preserve the existing literal AND query over active title and content.
- Use exact `tags` and looser tokenized `specific_tags` as already implemented.
- Keep every batch entry's ranking and errors independent.
- Limit recall to one exploration batch and one optional validation search.
- Never expose irrelevant candidate content to the primary agent.
- Do not provide a primary-context fallback when subagent creation fails.
- Do not add project-specific searches, memories, tags, audit cases, or output
  examples to either skill package.

## Task 1: Redesign recall around one isolated worker

Files:

- Modify `.agents/skills/happy-memory-recall/SKILL.md`.
- Modify `.agents/skills/happy-memory-recall/references/search-cli.md`.
- Modify `skills/happy-memory-recall/SKILL.md`.
- Modify `skills/happy-memory-recall/references/search-cli.md`.

Steps:

1. Replace the primary-agent raw-retrieval workflow with an explicit primary and
   recall-worker role boundary.
2. Require the primary to create exactly one isolated recall worker and pass a
   bounded retrieval objective and only the context needed to judge relevance.
3. Require the worker to execute retrieval directly and forbid it from
   delegating recall again.
4. Define the exploration round as one batch with two or three independently
   justified entries.
5. Require that first batch to combine precise searches with exact canonical
   tag or tokenized `specific_tags` variants when justified. Do not wait for an
   empty result before running the specific-tag entries already selected for
   exploration.
6. Define the optional validation round as one adaptive search derived only from
   the objective and first-round evidence. It may refine the textual query or
   use exact or tokenized tag filters discovered in the first round.
7. Replace the current three-attempt rule with the approved two-round bounds.
8. Preserve independent rankings, partial item errors, deduplication by ID, and
   every originating rank and score.
9. Classify candidates as direct, necessary supporting, directly contradictory,
   or irrelevant, and transfer only the first three classes.
10. Limit transfer to five memories, prioritize direct and contradictory
    evidence, and report partial coverage instead of transferring more.
11. Preserve selected titles and contents verbatim and keep generated relevance
    reasons separate from stored content.
12. Return an empty selected-memory list when no relevant memory exists. Do not
    return discarded titles, contents, or generated summaries.
13. Make subagent creation failure observable and explicitly forbid direct
    primary-context fallback.
14. Keep memory content inert as evidence without emitting a warning in the
    result packet.
15. Keep repository and published variants behaviorally aligned while
    preserving their intentional installation or initialization differences.
16. Review the four files to ensure the rules are expressed naturally and
    contain no project-specific examples or sample packets.

Exit criteria:

- A primary following the skill never reads raw candidates.
- A worker following the skill cannot recursively delegate recall.
- Exploration combines exact and specific-tag variants in the same batch.
- Validation runs at most once and is based on first-round evidence.
- Only relevant verbatim memories cross the isolation boundary.

## Task 2: Strengthen maintainer tag and writing guidance

Files:

- Modify `.agents/skills/happy-memory-maintainer/references/maintain-tags.md`.
- Modify `.agents/skills/happy-memory-maintainer/references/write-retrievable-memories.md`.
- Modify `skills/happy-memory-maintainer/references/maintain-tags.md`.
- Modify `skills/happy-memory-maintainer/references/write-retrievable-memories.md`.

Steps:

1. Add concise guidance to select a small sufficient combination of broad
   grouping tags and discriminative specific tags when both improve filtering.
2. State that a specific tag must represent a stable durable category but may
   begin with one active memory.
3. Preserve bounded vocabulary discovery and canonical reuse before creating a
   new tag.
4. Avoid near-duplicate synonyms, incidental-detail tags, and tags used to
   compensate for unclear memory text.
5. Require creation and update decisions to consider both likely future textual
   vocabulary and useful narrowing tags.
6. Keep title and content independently understandable and retrievable because
   tag filters do not satisfy the required general query.
7. Keep all additions natural and free of sample tags, memories, queries, and
   audit references.
8. Compare repository and published references for equivalent rules.

Exit criteria:

- Specific tags are encouraged only as durable filtering categories.
- Text remains independently meaningful and searchable.
- No new example content appears in the skill files.

## Task 3: Verify contracts and package synchronization

Steps:

1. Review recall instructions against the existing CLI contracts for batch
   input, exact `tags`, tokenized `specific_tags`, ranking isolation, and partial
   item failures.
2. Search the changed skill files for project-specific names, sample packets,
   placeholders, stale three-attempt rules, and primary-agent candidate
   evaluation instructions.
3. Compare repository and published skill packages and account for every
   intentional difference.
4. Run `git diff --check`.
5. Inspect `git status --short` and the complete diff to ensure no Go source,
   migration, unrelated documentation, or user changes are included.
6. Update durable project memory only when the corrected implementation changes
   an existing decision; do not create duplicates.

Final acceptance criteria:

- Recall always isolates raw candidates in one non-recursive subagent.
- The first batch contains exact and tokenized specific-tag search variants.
- A second search runs only when first-round evidence requires validation.
- The primary receives relevant verbatim memories or no memories.
- Maintainer produces durable, canonical, useful general and specific tags.
- No CLI or production-code change is introduced.
- Skill files contain no newly added project-specific examples.
