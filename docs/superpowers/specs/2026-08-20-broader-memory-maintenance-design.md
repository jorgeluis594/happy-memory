# Broader Memory Maintenance Design

## Goal

Allow `happy-memory-maintainer` to preserve a broader set of durable project
knowledge now that recall retrieves candidates in an isolated subagent and
transfers only relevant evidence to the primary agent.

The maintainer should store complete, atomic knowledge whose future usefulness
is confirmed or reasonably plausible. Immediate relevance to the current task
is not required. Honest importance and confidence scores express expected
impact and evidential strength without acting as admission thresholds.

## Eligibility Boundary

A candidate qualifies when it:

- is related to the project, product, architecture, workflow, or applicable
  project preferences;
- expresses one complete, context-independent idea;
- describes information expected to remain valid beyond the current task or
  conversation; and
- has confirmed or reasonably plausible reuse in a future task.

A candidate does not need to influence a known future decision or demonstrably
prevent rediscovery. Those properties can increase its importance but are not
required for persistence.

The maintainer continues to exclude:

- momentary progress and task state;
- incomplete fragments and raw conversation;
- incidental details without a reusable assertion;
- duplicate ideas and wording variants of adequate active memories; and
- unsupported speculation presented as fact.

Uncertain but coherent knowledge may qualify when its uncertainty is explicit
in the assertion and represented by an honest confidence score.

## Qualification and Maintenance Flow

At each natural consolidation point, the maintainer collects all complete
durable candidates related to the current work instead of retaining only the
ones with evident near-term impact.

For each candidate, it:

1. excludes only information outside the eligibility boundary;
2. separates ideas that can change, score, tag, or disappear independently;
3. searches related active memories to prevent duplication;
4. creates or updates the memory when reuse is reasonably plausible, even when
   expected impact is low;
5. assigns importance from the consequence of forgetting and confidence from
   the strength and currency of evidence; and
6. writes self-contained text and a minimal sufficient set of broad and
   discriminative tags.

Low importance and low confidence never cause automatic rejection after a
candidate satisfies the eligibility boundary. Scores remain independent:
importance does not represent certainty, and confidence does not represent
future value.

Atomicity, duplicate detection, tag vocabulary reuse, batch mutation, and error
handling retain their existing contracts. The change does not add a new memory
type, attribute, tag tier, CLI option, storage field, or ranking behavior.

## Skill Changes

Update both the repository-local and published copies of
`happy-memory-maintainer`.

The entrypoint should replace the strict requirement that every memory already
influence future decisions or prevent rediscovery with the broader durable and
plausibly reusable boundary. The creation guidance should apply the same
qualification rule. The scoring guidance should state directly that importance
and confidence describe qualifying memories and are not admission thresholds.

Other references change only when needed to remove a direct contradiction.
Instructions remain concise and natural, without project-specific examples.

## Recall Relationship

The relaxed maintenance threshold depends on the existing recall boundary, not
on a new storage behavior. Recall may retrieve broader candidate sets inside
its worker context, but only selected relevant memories cross to the primary
agent. This reduces the context cost of false-positive retrieval while leaving
the primary agent responsible for applying the selected evidence.

Recall isolation does not eliminate storage costs. The maintainer therefore
continues to reject transitory, incomplete, duplicate, and non-reusable content
and to maintain current knowledge when assertions evolve.

## Verification

Review and forward tests must verify that:

- complete durable project context with only plausible future utility qualifies;
- low importance does not independently disqualify a candidate;
- low confidence does not independently disqualify an explicitly uncertain
  candidate;
- momentary activity, incomplete fragments, incidental details, and duplicates
  remain excluded;
- no project-specific examples enter the skill instructions; and
- repository-local and published copies remain synchronized except for existing
  installation-specific guidance.

Run the skill validator for both maintainer directories after implementation.
