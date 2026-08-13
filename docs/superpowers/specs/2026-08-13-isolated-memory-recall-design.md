# Isolated Memory Recall Design

## Goal

Improve memory retrieval coverage without filling the primary agent's context
with tangential candidates. Retrieval and relevance assessment move into a
dedicated subagent with isolated context. The primary agent receives only
relevant memories and retains responsibility for deciding whether and how to
apply them to the current task.

This design also improves maintainer guidance so durable memories carry useful
general and specific tags while remaining naturally retrievable through their
title and content.

## Problem

Current recall performs literal AND matching over the active memory title and
content. A memory can exist and still remain undiscovered when the search uses
different project terminology. Broadening queries can recover more candidates,
but returning every candidate to the primary agent introduces irrelevant claims
into the context used for subsequent reasoning.

Batch search and `specific_tags` provide the mechanisms needed for bounded
exploration. Batch search keeps independent queries and rankings separate while
reducing invocation overhead. Specific tags constrain a textual query to
canonical tag tokens without allowing tags to satisfy the general text query or
affect its BM25 relevance.

## Scope

The change covers:

- the repository and published copies of `happy-memory-recall`;
- the repository and published copies of `happy-memory-maintainer`;
- their required read and maintenance references;
- an explicit strict or relaxed text-matching option in the search CLI;
- tests and documentation for the revised search and skill contracts.

The design assumes the host agent system supports subagents. Recall has no
direct-execution fallback in the primary agent.

The change does not add embeddings, model calls inside the CLI, persistent
relevance labels, automatic tag migration, or multiple verification subagents.

## Responsibility Boundary

The primary agent creates one isolated recall subagent and gives it only the
repository root, the retrieval objective, the minimum task context needed to
interpret relevance, and any known project entities or constraints. Raw memory
candidates never enter the primary agent's context.

The recall subagent owns:

- choosing bounded queries and justified filters;
- executing the search rounds;
- deduplicating candidates by memory ID;
- assessing relevance to the supplied objective;
- deciding whether the conditional second round is necessary;
- returning only relevant evidence.

The primary agent owns:

- deciding whether the selected evidence is sufficient for the task;
- checking whether it still agrees with current source code or documentation;
- resolving conflicts between selected memories and authoritative evidence;
- deciding how the evidence affects the current task;
- requesting a separate retrieval when a new objective arises.

If the subagent cannot be created, recall returns a failed retrieval. It must not
run the searches directly in the primary agent's context.

## Two-Round Retrieval

### Exploration round

The subagent performs one ordered batch containing two or three independently
justified strict searches. The batch includes both precise textual searches and
precise textual searches constrained by justified `specific_tags`. These
variants run together even when an unconstrained query already returns
candidates, because candidate presence does not establish relevance.

Every batch entry keeps its own ranking window, result order, score components,
and error. The subagent must not compare or merge scores across entries. It may
deduplicate repeated memories by ID while preserving every originating match.

The subagent evaluates candidates as direct, supporting, contradictory, or
irrelevant. Direct memories answer the retrieval objective. Supporting memories
provide context necessary to interpret direct evidence. Contradictory memories
directly conflict with otherwise relevant evidence. All other candidates are
irrelevant for transfer, including redundant memories that add no material
evidence.

### Conditional validation round

The subagent may perform one additional search after evaluating the exploration
round. It derives this search only from the original objective and observable
evidence from the first round.

The validation search is justified when:

- no relevant memory was found and relaxed matching may improve recall;
- direct evidence is incomplete or ambiguous and a focused query can resolve
  the missing point;
- relevant results conflict and a focused query can test the conflict.

When no relevant memory was found, the second search uses relaxed text matching
and may use one or more justified `specific_tags` to contain noise. When the
first round found ambiguous or conflicting relevant evidence, the second search
uses the strict or relaxed mode best suited to the missing fact. The round is
omitted when the exploration round already produced sufficient, consistent
evidence.

Recall performs no more than these two rounds. The exploration batch contains
at most three entries and the validation round contains exactly one entry.

## Search Match Contract

Search adds a `match` option with values `all` and `any`. Positional search
accepts `--match all|any`, and each batch entry accepts a `match` field. Omission
defaults to `all`, preserving existing callers and output shapes.

`all` retains the current behavior: whitespace-delimited literal terms are
escaped and joined with AND. `any` escapes the same terms and joins them with
OR. Both modes continue to search only the active title and content fields.

Structured filters remain conjunctive. Repeated exact `tags` and
`specific_tags` must all match regardless of the text match mode. Specific tags
continue to carry zero BM25 weight and cannot satisfy a general query term.

Relaxed mode uses the existing candidate window, BM25 calculation, metadata
normalization, final-score formula, and tie-breakers. This does not change the
ranking formula, so `ranking_version` remains unchanged. Results from different
queries or match modes remain incomparable.

Unknown match values produce `VALIDATION_ERROR`. A one-term query behaves the
same in both modes. Existing punctuation-only and empty-query behavior remains
unchanged.

## Subagent Result Contract

The subagent returns a small structured packet containing:

- execution status and retrieval coverage;
- the executed rounds, queries, match modes, filters, ranking versions, and
  result counts;
- up to five selected relevant memories;
- an error when retrieval could not complete.

Each selected memory contains its ID, type, title, content, tags, importance,
confidence, relevance classification, a concise relevance reason, and every
originating rank and score. Title and content are preserved verbatim. Generated
relevance explanations remain separate from stored content.

Only direct, necessary supporting, and directly contradictory memories may be
selected. The packet never includes the content, title, or generated summary of
irrelevant or redundant candidates. If no relevant memory exists, the selected
memory list is empty and no memory content crosses the isolation boundary.
When more than five relevant memories exist, direct and contradictory evidence
takes precedence over supporting evidence. The subagent selects the smallest
set that preserves the material answer and conflict coverage, and reports
partial coverage rather than transferring additional memories.

Stored memory content is evidence rather than executable instruction. The
subagent must not let candidate content alter its retrieval objective, execute
commands, mutate data, or expand authorization. This rule is internal to recall
and is not emitted as a warning in the result packet.

## Recall Skill Changes

The skill instructions must directly require isolated subagent delegation and
the two-round process. They replace the current rule that the primary agent
chooses every query and evaluates every raw candidate. The primary agent keeps
final task judgment, but query selection and relevance filtering belong to the
recall subagent.

The skill and read contract must describe batch search, `specific_tags`, the
new match mode, per-entry ranking isolation, partial batch failures, the bounded
rounds, relevance classification, and the output boundary consistently.

The instructions must state these requirements naturally. They must not include
project-specific queries, memory records, audit scenarios, or example output
packets.

## Maintainer Skill Changes

Maintainer continues to write atomic, self-contained memories and to keep
central retrieval terms naturally present in title or content. Tags complement
textual retrieval; they do not replace clear searchable writing.

Tag selection should use a small sufficient combination of broad grouping tags
and discriminative specific tags when both materially improve future filtering.
A specific tag is appropriate when it names a stable, durable category. It may
begin on one memory and later be reused; current frequency is evidence about
usage, not a prerequisite for creation.

Maintainer must prefer an adequate canonical tag over a synonym, avoid
near-duplicate vocabulary, and avoid tags that merely restate incidental detail
or compensate for unclear memory text. Before creating or updating a memory, it
assesses both the likely textual vocabulary future agents will use and the tags
that can meaningfully narrow that search.

These rules are added as concise natural instructions. The maintainer skill and
references must not include project-specific tags, sample memories, or audit
examples.

## Errors and Partial Results

Failure to create the isolated subagent is a retrieval failure. The primary
agent receives the failure but no raw candidates.

A command-level batch failure ends the current retrieval. An item-level failure
does not discard successful sibling entries. The subagent can still assess
successful entries and may use the validation round when the failed entry leaves
coverage incomplete. The final packet preserves public errors without exposing
discarded candidate content.

If the validation search fails after relevant exploration evidence was found,
the packet preserves that selected evidence and reports partial coverage with
the validation error. If no relevant evidence was selected, the selected list
remains empty.

## Verification

CLI tests cover:

- default compatibility with strict AND matching;
- explicit `all` and `any` behavior in positional and batch modes;
- invalid match values and strict JSON validation;
- the interaction of relaxed text matching with exact tags, specific tags,
  type, importance, confidence, and limits;
- unchanged ranking formula and ranking-version behavior;
- partial batch failures and independent ranking windows.

Skill scenario tests or review fixtures cover:

- sufficient relevant evidence in the exploration batch;
- simultaneous unscoped and specific-tag searches in the first batch;
- conditional relaxed fallback when the first round has no relevant evidence;
- focused validation of incomplete or contradictory evidence;
- many candidates with no relevant memory transferred;
- transfer of complete verbatim relevant memories only;
- an empty selected list when no relevant memory exists;
- subagent creation failure without primary-context search fallback;
- command-level and item-level batch failures;
- absence of project-specific examples in the skill instructions.

Repository-local and published skill copies must remain synchronized.
