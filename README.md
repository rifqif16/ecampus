# E-Campus

Proyek portfolio fullstack + UI/UX: sistem akademik kampus yang informatif dan proaktif.

| Komponen    | Teknologi                       | Direktori  |
| ----------- | ------------------------------- | ---------- |
| Kontrak API | OpenAPI 3.0.3 (spec-first)      | `api/`     |
| Backend     | Go, chi, pgx + sqlc, PostgreSQL | `backend/` |
| Web         | React + Vite (SPA)              | `web/`     |
| Mobile      | Flutter                         | `mobile/`  |
| Deploy      | Docker Compose + Caddy          | `deploy/`  |

Status: MVP (R1). Spesifikasi lengkap: `docs/ecampus.md`.

## Konvensi

- Commit: Conventional Commits.
- Branch: `main` + `feature/<phase-name>`.
- Struktur file: `FILE_MANIFEST.md`.
- Keputusan arsitektural: `docs/adr/`.
