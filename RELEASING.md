# Releasing

This project uses:

- `CHANGELOG.md` as the source of truth for release notes
- Git tags such as `v0.1.0`
- GitHub Releases for published versions

## How `Unreleased` works

`Unreleased` is the working area for the next version.

During normal development, add changes there first:

```md
## [Unreleased]

### Added
- add PostgreSQL provider helper

### Changed
- refine README onboarding

### Fixed
- avoid `= NULL` in selective predicates
```

When you are ready to publish, move those entries into a real version section
and keep a fresh empty `Unreleased` block at the top.

## Demo: from `v0.1.0` to `v0.1.1`

### Before release

`CHANGELOG.md`:

```md
## [Unreleased]

### Added
- add release guide

### Changed
- improve app wiring examples

### Fixed
- fix typo in README

## [0.1.0] - 2026-03-30
...
```

### Prepare `v0.1.1`

Change it to:

```md
## [Unreleased]

### Added

### Changed

### Fixed

## [0.1.1] - 2026-04-02

### Added
- add release guide

### Changed
- improve app wiring examples

### Fixed
- fix typo in README

## [0.1.0] - 2026-03-30
...
```

Then commit the changelog update:

```bash
git add CHANGELOG.md
git commit -m "docs: prepare v0.1.1 release"
git push origin master
```

## Tag and publish

After the changelog commit is on `master`, create and publish the tag:

```bash
git tag -a v0.1.1 -m "Release v0.1.1"
git push origin v0.1.1
gh release create v0.1.1 --title "v0.1.1" --generate-notes
```

For a first release the flow is the same, just start from `v0.1.0`.

## Compare two releases

Useful local commands:

```bash
git log --oneline v0.1.0..v0.1.1
git diff --stat v0.1.0..v0.1.1
git diff --name-status v0.1.0..v0.1.1
git diff v0.1.0..v0.1.1 -- README.md
```

Examples:

- Show commits added after `v0.1.0`
- Show which files changed between `v0.1.0` and `v0.1.1`
- Inspect one file's exact diff across releases

You can also use GitHub's compare page:

```text
https://github.com/dennisge/sqlcraft/compare/v0.1.0...v0.1.1
```

## Suggested release rhythm

- Patch release like `v0.1.1`: documentation fixes, bug fixes, small safe improvements
- Minor release like `v0.2.0`: new features that keep compatibility
- Major release like `v1.0.0`: first stable API promise

Before `v1.0.0`, it is normal to keep iterating on API shape.

## Checklist

Before release:

- `CHANGELOG.md` moved from `Unreleased` into a dated version section
- `go test ./...`
- `go vet ./...`
- `master` pushed

Publish:

- create annotated tag
- push tag
- create GitHub Release

After release:

- keep `Unreleased` empty and ready for the next cycle
