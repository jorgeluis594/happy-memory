# Repositories

Repositories persist and retrieve domain state. They contain no business logic.

## Boundaries

- Implement ports declared by the consuming domain.
- Place implementations in `internal/adapters/<technology>/`.
- Accept and return domain contract types; do not expose GORM types outside the adapter.
- Keep persistence row models separate from domain entities and map explicitly between them.
- Limit responsibilities to query composition, persistence, mapping, transactions, and technical error translation.

## Use GORM

- Store a `*gorm.DB` dependency and perform repository database work through GORM.
- Attach `context.Context` with `WithContext(ctx)` to every operation.
- Use parameterized GORM queries, explicit filters, and deterministic ordering required by the port.
- Use `db.Transaction(func(tx *gorm.DB) error { ... })` for atomic multi-write operations and use only `tx` inside the callback.
- Detect `gorm.ErrRecordNotFound` with `errors.Is` and translate it according to the domain contract.
- Translate other database and constraint errors without leaking GORM errors across the port.
- Use GORM `Raw` or `Exec` only when the regular query APIs cannot express the operation clearly.
- Manage schema changes with versioned Goose migrations. Never use `AutoMigrate`.

## Exclude Business Logic

Do not validate domain meaning, normalize domain values, calculate business data, choose defaults, enforce policies, or decide state transitions in a repository or GORM hook. Inputs arrive already governed by the domain. Implement filters and ordering exactly as required by the port without inventing policy.

Test repositories with integration tests against SQLite, covering mappings, queries, transactions, not-found behavior, constraints, and error translation.
