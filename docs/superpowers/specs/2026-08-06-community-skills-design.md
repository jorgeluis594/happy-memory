# Community skills distribution design

## Goal

Publish the existing `happy-memory-recall` and `happy-memory-maintainer` workflows from this repository in a layout discoverable by the `npx skills` CLI, without changing their repository-local copies.

## Structure

Create two self-contained skill directories:

```text
skills/
├── happy-memory-maintainer/
│   ├── SKILL.md
│   ├── agents/openai.yaml
│   └── references/
└── happy-memory-recall/
    ├── SKILL.md
    ├── agents/openai.yaml
    └── references/
```

Initialize each directory with `npx skills init <name>`, then replace the generated template with an exact copy of the corresponding directory under `.agents/skills/`.

## Boundaries

- Keep `.agents/skills/happy-memory-recall` and `.agents/skills/happy-memory-maintainer` unchanged.
- Copy every supporting file so each published skill remains self-contained.
- Do not publish the other repository-only skills.
- Do not change CLI production code or runtime behavior.

## Validation

1. Compare each published directory with its `.agents/skills/` source.
2. Validate both `SKILL.md` files with the skill validation tooling when available.
3. Run `npx skills add . --list` and confirm both skills are discovered.
4. Review `git diff` to ensure the change contains only the two published copies and this design document.

## Maintenance trade-off

The published copies can drift from the repository-local versions. For this initial change, exact directory comparison is the synchronization check; automation is outside scope.
