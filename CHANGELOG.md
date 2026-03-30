# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
once tagged releases begin.

## [Unreleased]

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
