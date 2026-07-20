# Move config to internal/ Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move `config/` to `internal/config/` so all packages live under `internal/`, consistent with the rest of the codebase.

**Architecture:** `git mv` the package directory, update all 19 import sites from `github.com/user/kb/config` to `github.com/user/kb/internal/config`, verify build and tests pass.

**Tech Stack:** Go 1.26, standard `go build` / `go test`

## Global Constraints

- Module path: `github.com/user/kb`
- New import path: `github.com/user/kb/internal/config`
- Package name stays `config` — no renames
- No behaviour changes — pure mechanical refactor

---

### Task 1: Move the package directory and fix all import paths

**Files:**
- Move: `config/` → `internal/config/`
- Modify (import path only): `internal/config/config_test.go`
- Modify (import path only): `internal/provider/provider.go`
- Modify (import path only): `internal/provider/openai/openai.go`
- Modify (import path only): `internal/provider/openai/openai_test.go`
- Modify (import path only): `internal/provider/azure/azure.go`
- Modify (import path only): `internal/provider/azure/azure_test.go`
- Modify (import path only): `internal/provider/oauthopenai/provider.go`
- Modify (import path only): `internal/provider/oauthopenai/provider_test.go`
- Modify (import path only): `internal/adapters/confluence/confluence.go`
- Modify (import path only): `internal/adapters/confluence/confluence_test.go`
- Modify (import path only): `internal/adapters/file/file.go`
- Modify (import path only): `internal/adapters/file/pdf_vision_test.go`
- Modify (import path only): `internal/embedder/embedder.go`
- Modify (import path only): `internal/embedder/openai/openai.go`
- Modify (import path only): `internal/embedder/openai/openai_test.go`
- Modify (import path only): `internal/cmd/root.go`
- Modify (import path only): `internal/cmd/status.go`
- Modify (import path only): `internal/cmd/ingest.go`
- Modify (import path only): `internal/cmd/repair.go`

**Interfaces:**
- Consumes: nothing (first task)
- Produces: `github.com/user/kb/internal/config` package with identical API

- [ ] **Step 1: Move the directory**

```bash
git mv config internal/config
```

- [ ] **Step 2: Update all import paths in one shot**

```bash
find /root/workspace/kb -name "*.go" | xargs sed -i 's|github.com/user/kb/config|github.com/user/kb/internal/config|g'
```

- [ ] **Step 3: Verify no old import path remains**

```bash
grep -r "github.com/user/kb/config" /root/workspace/kb --include="*.go"
```

Expected: no output (zero matches).

- [ ] **Step 4: Verify build succeeds**

Run from repo root:
```bash
go build ./...
```

Expected: exits 0, no errors.

- [ ] **Step 5: Run all tests**

```bash
go test ./...
```

Expected: all packages pass, exit 0.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor: move config package to internal/config"
```
