# Task 9 Report: CLI Commands

## Status: DONE

## Commits
- `ae5542d` feat: CLI commands - ingest, search, serve, status, config

## Build/Smoke-Test Summary
`CGO_ENABLED=1 go build -o kb .` succeeded; `./kb --help`, `./kb search --help`, and `./kb ingest --help` all show correct subcommands and flags.

## Concerns
- `cmd/kb/main.go` was updated per the brief but references `rootCmd` which is defined in root-level package. Since `go build -o kb .` runs from the root, the root-level `main.go` + `cmd_*.go` files form the actual binary. The updated `cmd/kb/main.go` would fail to build standalone (`rootCmd` undefined), but this is consistent with the brief's intent that `cmd/kb/` is no longer the build entry point.
- `flagForce` is registered on both `ingestFileCmd` and `ingestConfluenceCmd` as separate flags (not shared variable conflict) — this is fine since the brief specified it that way.
- `runSource` in `cmd_ingest.go` accesses `src.Extensions[0]` without a nil/empty check; if a file source is stored with an empty Extensions slice it will panic. This follows the brief exactly and can be hardened later.

## Report File
/root/workspace/kb/.superpowers/sdd/task-9-report.md

---

## Fix Pass (Task 9 Issues)

### Fixes Applied

1. **Deleted `cmd/kb/main.go`** — The file referenced `rootCmd` from the root package, which is undefined in that package context. Since the binary entry point is the root-level `main.go`, `cmd/kb/` was deleted entirely (`cmd/kb/main.go`, `cmd/kb/`, `cmd/`).

2. **Fixed `runSource` Extensions guard** (`cmd_ingest.go:93`) — Was panicking on `src.Extensions[0]` when slice was empty. Now uses a safe default of `["md","txt","pdf"]` and only accesses the slice when `len > 0`.

3. **Fixed `config.Save` error handling** (`cmd_ingest.go:registerSource`) — Both calls to `config.Save(cfg)` now check and report errors via `fmt.Fprintf(os.Stderr, ...)`.

4. **Fixed `truncate` rune slicing** (`cmd_search.go:85`) — Now uses `[]rune(s)` to correctly handle multi-byte UTF-8 characters instead of byte-slicing.

### Build Verification
- `CGO_ENABLED=1 go build -o kb .` — PASS (no output)
- `CGO_ENABLED=1 go build ./...` — PASS
- `./kb --help` — shows all commands correctly
- `./kb ingest --help` — shows file and confluence subcommands
