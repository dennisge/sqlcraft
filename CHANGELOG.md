# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
once tagged releases begin.

## [Unreleased]

### Added

### Changed

### Fixed

## [0.2.0] - 2026-03-30

### Added

- `WithContext(ctx)` and `TransactionContext(ctx, ...)` for request-scoped queries and transactions.
- Provider-controlled `NewTxSession(tx)` helpers for native `database/sql` transactions in the MySQL and PostgreSQL driver packages.
- Expanded transaction coverage in unit tests and real-database live checks for commit and rollback behavior.
- Production-oriented wiring examples and a release workflow guide in the documentation.

### Changed

- Simplified the recommended transaction usage in the README to prefer `session.Transaction(...)` and provider-specific control only when needed.
- Preserved session debug/context state when entering callback-based transactions.
- Improved API documentation around request DTO usage, provider wiring, and transaction behavior.

### Fixed

- Corrected GORM-backed transaction execution so MySQL inserts no longer escape the active transaction when collecting generated IDs.
- Added panic-safe rollback handling for the `database/sql` transaction path.
- Propagated execution context through native `database/sql` queries, execs, and transaction begin calls.

## [0.1.0] - 2026-03-30

### Added

- Pluggable execution providers for both GORM and native `database/sql`.
- Provider-specific driver helpers for MySQL and PostgreSQL.
- `ExecResult()` and `Returning(...)` support for generated ID workflows.
- Presence-aware `Optional[T]`, `Present(...)`, `Absent(...)`, and `Maybe(...)` helpers for API DTOs.
- `AppendRawNamed(...)` and condition-pruning `AppendRawSelective(...)` helpers for complex dynamic SQL.
- Real-database live checks for MySQL and PostgreSQL under `cmd/livecheck`.
- GitHub Actions CI covering formatting, unit tests, vet, and live DB verification.

### Changed

- Simplified session entry points so application code can prefer provider packages over explicit dialect wiring.
- Improved README onboarding with API DTO examples, raw SQL guidance, and generated ID documentation.
- Normalized session error messages and expanded API documentation.

### Fixed

- Avoided silent `= NULL` behavior in Selective equality predicates by pruning explicit null optionals.
- Preserved `BETWEEN ... AND ...` predicates during `AppendRawSelective(...)` pruning.
- Improved parameter rebinding and placeholder handling for appended SQL fragments.
