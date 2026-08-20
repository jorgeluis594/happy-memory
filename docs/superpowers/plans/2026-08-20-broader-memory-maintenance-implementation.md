# Broader Memory Maintenance Implementation Plan

## Objective

Implement the approved broader eligibility boundary from
`docs/superpowers/specs/2026-08-20-broader-memory-maintenance-design.md` in the
repository-local and published copies of `happy-memory-maintainer`.

No CLI, storage, schema, ranking, recall, or memory-type change is required.

## Task 1: Broaden the entrypoint eligibility rule

Modify:

- `.agents/skills/happy-memory-maintainer/SKILL.md`
- `skills/happy-memory-maintainer/SKILL.md`

Replace the requirement for demonstrated future influence and prevented
rediscovery with confirmed or reasonably plausible future reuse. State that
low importance or confidence does not independently disqualify an otherwise
eligible memory. Preserve the exclusions for momentary state and incomplete or
non-reusable content.

## Task 2: Align creation and scoring guidance

Modify:

- `.agents/skills/happy-memory-maintainer/references/create-memories.md`
- `skills/happy-memory-maintainer/references/create-memories.md`
- `.agents/skills/happy-memory-maintainer/references/score-memories.md`
- `skills/happy-memory-maintainer/references/score-memories.md`

Make creation qualify complete, atomic, durable candidates whose reuse is
confirmed or reasonably plausible. Keep duplicate detection unchanged. Clarify
that importance and confidence describe qualifying memories rather than gate
their persistence, and require uncertainty to remain explicit in stored text.

Do not add project-specific examples, new metadata, or a second memory tier.

## Task 3: Verify the skill package

1. Compare the repository-local and published changed files.
2. Run `quick_validate.py` for both maintainer directories.
3. Review the final diff for contradictory strict eligibility wording,
   placeholders, unintended examples, and unrelated changes.
4. Forward-test the eligibility boundary with an independent subagent if the
   final wording leaves meaningful behavioral ambiguity.

## Task 4: Consolidate durable project knowledge

After validation, update project memory with the approved maintainer policy.
Use the maintainer's normal duplicate search and batch mutation contract, and
preserve any existing recall-isolation decision as a separate atomic memory.
