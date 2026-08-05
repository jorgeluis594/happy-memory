---
name: implement-code
description: "Implement, modify, refactor, or fix code in happy-memory through validated incremental groups of at most five production code files. Use whenever Codex will create, edit, delete, or otherwise touch production code, including feature work, refactoring, bug fixes, and code maintenance."
---

# Implement Code Incrementally

Work in coherent groups that remain small enough to validate before continuing.

## Prepare

- Inspect the requirements, architecture, and `git status --short` before editing.
- Preserve unrelated user changes.
- Read [business-logic.md](references/business-logic.md) before changing domain rules or use cases.
- Read [repositories.md](references/repositories.md) before changing repository or persistence code.
- Read both references when a group crosses both concerns.
- Inventory the required production files and partition them into coherent groups of at most five.

## Count Production Files

- Count every production source file created, modified, or deleted in a group.
- Do not count tests, migrations, or documentation.
- Count a file once within its group, including fixes made during validation.
- Put newly discovered production files in the current group only when the total remains at most five; otherwise repartition the remaining work.
- Never exceed the limit. Reshape the increment when more than five production files appear inseparable.

## Implement and Validate Each Group

1. State the group's outcome and its production files.
2. Implement only that group, including its tests and required migration or documentation updates.
3. Format the changed code.
4. Run the focused tests that cover the changed behavior.
5. Run `make lint`.
6. Fix failures attributable to the group and repeat both tests and lint.
7. Start the next group only after both validations pass. If a pre-existing failure prevents that, report the evidence and stop.

Keep corrections inside the current group. Do not use the next group to finish or repair an incomplete one.

## Finish

- Run `make check` after all groups pass their focused validation.
- Review the final diff for unintended changes and formatting errors.
- Report each group, its production files, and the test, lint, and final-check results.
