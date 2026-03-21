# Infrastructure

Living spec for Docker, CI/CD, and operational decisions.

## Key Decisions

- **Docker: system user (`-S`), `/data` dir created in Dockerfile** — No home dir or login shell. `chown` ensures writability with or without volume mount. (from GH-9)

- **Dependabot: weekly schedule for both ecosystems** — Daily is too noisy for a small project. (from GH-12)

- **CodeQL over third-party scanners** — Free, first-party, no external accounts. Covers Go-specific vulnerability patterns. (from GH-12)

- **Branch protection documented, not automated** — Requires admin access to repo settings UI/API, can't be configured via committed files. (from GH-12)

- **CI: `go test` workflow on PRs and pushes to main** — Complements CodeQL by catching functional regressions. (from GH-16)

- **PR-label-based semver** — `major`, `minor`, `patch` labels on PRs drive automated version tagging on merge. No label = no release. (from GH-16)

- **Docker-only release over GoReleaser** — GoReleaser adds config complexity and binary distribution overhead. `docker/build-push-action` is simpler; Docker is the sole distribution method. (from GH-16)

- **GitHub Release with auto-generated changelog** — `gh release create --generate-notes` on each version tag. No binary attachments. (from GH-16)
