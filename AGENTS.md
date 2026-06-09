# AGENTS.md

This repository includes an AI-agent usage package for `ginx`.

## For AI Agents

When asked to build or modify Go HTTP backend services with this library, read:

- [skills/ginx-http-backend/SKILL.md](skills/ginx-http-backend/SKILL.md)
- [README.md](README.md) for the runtime main path
- [README_CODEGEN.md](README_CODEGEN.md) for the OpenAPI/oapi-ginx main path

The skill is the shortest operational guide. The two root README files are intentionally concise. Use [docs/RUNTIME_REFERENCE.md](docs/RUNTIME_REFERENCE.md) and [docs/CODEGEN_REFERENCE.md](docs/CODEGEN_REFERENCE.md) only when a task needs detailed API/reference behavior.

## Recommended Workflow

1. Decide whether the task is runtime-only or OpenAPI/codegen-based.
2. Use `context.Context + *Req -> (*Rsp, error)` handlers.
3. Configure a dedicated `ginx.Engine` before route registration.
4. Prefer `WithStrictJSONBody(true)` and internal error masking for public services.
5. If `oapi-ginx.yaml` exists, generate from spec and implement `ServerInterface`; do not hand-edit generated files.
6. Run `go test ./... -count=1`; for codegen/template/spec changes also run `./scripts/test-codegen-e2e.sh`.

## Using The Skill Outside This Repo

The directory `skills/ginx-http-backend/` is a portable Codex-style skill. A user or maintainer can copy that directory into their agent skill registry and invoke it as `$ginx-http-backend`.
