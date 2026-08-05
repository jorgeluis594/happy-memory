# Business Logic

Business logic expresses the rules and decisions that give the domain its meaning.

## What Belongs in Business Logic

- Invariants and allowed state transitions.
- Semantic validation of domain inputs.
- Domain-significant normalization.
- Calculations, policies, priorities, and decisions.
- Use-case coordination.
- Domain errors and stable error codes.

## How to Implement It

- Keep it in `internal/<domain>/`, never in adapters, `internal/app`, or `cmd`.
- Keep domain entities, values, and contracts independent of GORM and transport types.
- Prefer pure functions for deterministic rules and entity methods for invariant-preserving behavior.
- Use a service or operation-focused use case to coordinate entities and ports.
- Declare ports from the consuming domain's perspective and expose domain types in their contracts.
- Separate semantic validation from parsing and presentation validation.
- Return explicit domain errors instead of technology-specific errors.
- Test rules, boundaries, transitions, and error cases with unit tests.
- Add abstractions or files only when an existing responsibility requires them.

## What Does Not Belong

Database queries, row mapping, serialization, CLI parsing, framework configuration, dependency wiring, and generic I/O mechanics are not business logic. Keep them in their corresponding adapters or application composition layer.
