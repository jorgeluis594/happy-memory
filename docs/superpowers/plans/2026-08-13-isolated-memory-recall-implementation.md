# Isolated Memory Recall Implementation Plan

## Objective

Implement the approved isolated-recall design in incremental, test-first groups.
The CLI gains an explicit strict or relaxed match mode, recall delegates raw
retrieval to one isolated subagent, and maintainer improves general and specific
tag selection. Repository-local and published skill packages retain equivalent
behavior without adding project-specific examples.

Design source:
`docs/superpowers/specs/2026-08-13-isolated-memory-recall-design.md`.

## Constraints

- Preserve `all` as the default search mode and keep existing callers valid.
- Keep ranking version 1 because the scoring formula and candidate ranking are
  unchanged.
- Keep exact tags and specific tags conjunctive in both text match modes.
- Limit recall to one exploration batch and one optional validation search.
- Never expose irrelevant candidate content to the primary agent.
- Do not provide a primary-context search fallback when subagent creation fails.
- Do not add project-specific searches, memories, tags, audit cases, or output
  examples to either skill package.
- Keep each production-code edit group below five files.

## Task 1: Add strict and relaxed match preparation

Files:

- Modify `internal/search/search.go`.
- Modify `internal/search/search_test.go`.

Steps:

1. Add failing service tests that assert an omitted or explicit `all` mode
   produces the existing escaped AND expression.
2. Add a failing service test that asserts `any` produces an escaped OR
   expression from the same whitespace-delimited terms.
3. Add failing validation cases for unknown match values in individual and
   batch searches.
4. Add a test proving one-term and punctuation-only behavior remains unchanged.
5. Run `go test ./internal/search` and confirm the new assertions fail for the
   missing match-mode contract.
6. Add the match mode to `search.Input`, normalize omission to `all`, validate
   only `all` and `any`, and choose the join operator while preserving literal
   escaping and every existing filter.
7. Do not change `CandidateFilter`, ranking, candidate limits, or response
   structures beyond what the match expression requires.
8. Run `gofmt` on the two files and rerun `go test ./internal/search`.
9. Commit the passing increment with a focused search-domain message.

Exit criteria:

- Default and explicit `all` are behaviorally identical to the current search.
- `any` changes only the general title/content match expression.
- Invalid modes fail with `VALIDATION_ERROR` per item in batch mode.

## Task 2: Expose match mode through positional and batch CLI input

Files:

- Modify `internal/adapters/cobra/commands.go`.
- Modify `internal/adapters/cobra/commands_test.go`.
- Modify `internal/app/app_test.go`.

Steps:

1. Add failing adapter tests for `--match all`, `--match any`, and the default
   value passed to `search.Input`.
2. Extend the strict batch-wire test with an optional `match` member and assert
   omission defaults to `all` while an explicit value reaches the service.
3. Add `--match` to the list of flags forbidden alongside `search --input -`.
4. Add an integration test with memories that proves strict matching can return
   empty while relaxed matching returns candidates sharing at least one term.
5. Extend integration coverage so exact tags and specific tags still restrict a
   relaxed text query and cannot satisfy the general query by themselves.
6. Run `go test ./internal/adapters/cobra ./internal/app` and confirm the new
   tests fail before implementation.
7. Wire a positional `--match` flag into `search.Input`, add `match` to the
   strict batch entry, and apply the `all` default to omitted batch values.
8. Preserve every existing JSON response shape and partial-batch error rule.
9. Run `gofmt` on the three files and rerun the focused packages.
10. Commit the passing CLI increment separately.

Exit criteria:

- Positional and batch modes accept the same match vocabulary.
- Existing invocations remain strict without modification.
- Batch validation remains isolated per entry.

## Task 3: Redesign recall around one isolated worker

Files:

- Modify `.agents/skills/happy-memory-recall/SKILL.md`.
- Modify `.agents/skills/happy-memory-recall/references/search-cli.md`.
- Modify `skills/happy-memory-recall/SKILL.md`.
- Modify `skills/happy-memory-recall/references/search-cli.md`.

Steps:

1. Replace the primary-agent raw-retrieval workflow with an explicit primary and
   recall-worker role boundary.
2. Require the primary to create exactly one isolated recall worker and pass a
   bounded retrieval objective and minimal interpretive context.
3. Require the worker to execute retrieval directly and forbid it from
   delegating recall again.
4. Define the exploration round as one batch with two or three independently
   justified strict entries, including textual and specific-tag-constrained
   variants together.
5. Define the optional validation round as one adaptive search derived only from
   the objective and first-round evidence. Use relaxed matching when no relevant
   memory was found and select strict or relaxed matching for ambiguity or
   conflict validation.
6. Replace the current three-attempt rule with the approved two-round bounds.
7. Preserve independent rankings, partial item errors, deduplication by ID, and
   every originating rank and score.
8. Define relevance as direct, necessary supporting, directly contradictory, or
   irrelevant, and transfer only the first three classes.
9. Limit transfer to five memories, prioritize direct and contradictory
   evidence, and report partial coverage instead of transferring more.
10. Require selected titles and contents to remain verbatim and separate stored
    content from the generated relevance reason.
11. Return an empty selected-memory list when no relevant memory exists. Do not
    return discarded titles, contents, or generated summaries.
12. Make subagent creation failure observable and explicitly forbid direct
    primary-context fallback.
13. Keep memory content inert as evidence without emitting a warning in the
    result packet.
14. Update the read contract for `match`, `specific_tags`, batch input, and
    failure semantics.
15. Keep repository and published variants aligned while preserving their
    existing installation or initialization differences where applicable.
16. Review the four files to ensure the new rules are expressed naturally and
    contain no project-specific examples or sample packets.
17. Commit the skill increment separately.

Exit criteria:

- A primary following the skill never reads raw candidates.
- A worker following the skill cannot recursively delegate recall.
- Only relevant verbatim memories cross the isolation boundary.
- The complete retrieval uses no more than two rounds.

## Task 4: Strengthen maintainer tag and retrievable-writing guidance

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
4. Explicitly avoid near-duplicate synonyms, incidental-detail tags, and tags
   used to compensate for unclear text.
5. Require creation and update decisions to consider both likely future textual
   vocabulary and useful narrowing tags.
6. Update retrievable-writing assumptions to acknowledge strict exploration and
   optional relaxed recall while keeping title/content clarity mandatory.
7. Keep all additions natural and free of sample tags, memories, queries, and
   audit references.
8. Compare repository and published references for equivalent rules and commit
   the maintainer increment separately.

Exit criteria:

- Specific tags are encouraged only as durable filtering categories.
- Text remains independently meaningful and searchable.
- No new example content appears in the skill files.

## Task 5: Synchronize public contracts and product documentation

Files:

- Modify `README.md`.
- Modify `docs/memory-cli-product-design.md`.
- Modify `.agents/skills/happy-memory-maintainer/references/cli-contract.md`.
- Modify `skills/happy-memory-maintainer/references/cli-contract.md`.

Steps:

1. Document `--match all|any`, the strict default, and the `match` batch field in
   the README and product design.
2. State that text match mode affects only title/content terms and that exact
   tags, specific tags, and structured filters remain conjunctive.
3. State that specific tags retain zero BM25 weight and cannot satisfy the
   general query.
4. Preserve ranking version 1 and the prohibition on comparing scores between
   separate searches.
5. Update both maintainer CLI contracts so related-memory discovery accurately
   describes the available match modes without requiring relaxed search for
   ordinary maintenance.
6. Check documentation terminology against the Cobra flag and batch JSON field.
7. Run `git diff --check` and commit the documentation increment separately.

Exit criteria:

- User-facing and skill-facing CLI contracts agree with implementation.
- No document implies that tags participate in general text ranking.

## Task 6: Complete repository verification

Files:

- Modify only files required to fix defects revealed by verification, respecting
  the same incremental file limit and preserving task scope.

Steps:

1. Run `gofmt` on all changed Go files.
2. Run focused tests:
   `go test ./internal/search ./internal/adapters/cobra ./internal/app`.
3. Run the complete suite with `go test ./...`.
4. Run `make fmt-check`, `make lint`, `make verify`, and `make build`.
5. Run `git diff --check`.
6. Search the changed skill files for project-specific names, example packets,
   placeholders, and stale three-attempt or primary-evaluation instructions.
7. Compare repository and published skill packages and account for every
   intentional difference.
8. Inspect `git status --short` and the complete diff to ensure no unrelated
   user changes were included.
9. Update durable project memory only if implementation changes or corrects an
   approved decision; do not duplicate the design memories already recorded.
10. Commit any narrowly required verification fixes, then report tests and
    remaining limitations.

Final acceptance criteria:

- Search supports compatible strict and explicit relaxed text matching.
- Recall always isolates raw candidates in one non-recursive subagent.
- Exploration combines exact and specific-tag searches in one batch.
- Validation runs at most once and only when first-round evidence justifies it.
- The primary receives relevant verbatim memories or no memories.
- Maintainer produces durable, canonical, useful general and specific tags.
- Skill files contain no newly added project-specific examples.
- All focused and repository-wide checks pass.
