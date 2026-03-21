# Infrastructure

Living spec for Docker, CI/CD, and operational decisions.

## Key Decisions

- **Docker: system user (`-S`), `/data` dir created in Dockerfile** — No home dir or login shell. `chown` ensures writability with or without volume mount. (from GH-9)

- **Dependabot: weekly schedule for both ecosystems** — Daily is too noisy for a small project. (from GH-12)

- **CodeQL over third-party scanners** — Free, first-party, no external accounts. Covers Go-specific vulnerability patterns. (from GH-12)

- **Branch protection documented, not automated** — Requires admin access to repo settings UI/API, can't be configured via committed files. (from GH-12)
