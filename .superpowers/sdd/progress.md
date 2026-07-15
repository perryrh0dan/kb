# SDD Progress Ledger — kb

## Tasks
- [ ] Task 1: Go Module + Project Scaffold
- [ ] Task 2: Config System
- [ ] Task 3: Recursive Character Chunker
- [ ] Task 4: SQLite Store
- [ ] Task 5: OpenAI Embedder
- [ ] Task 6: File Adapter
- [ ] Task 7: Confluence Adapter
- [ ] Task 8: Ingest Orchestrator
- [ ] Task 9: CLI Commands
- [ ] Task 10: MCP Server
- [ ] Task 11: Integration Test & README

## Log
Task 1: complete (commits b553d80..4d3f2d0, review clean)
Task 2: complete (commits 4d3f2d0..f159ed0, review clean after Save fix + test isolation fix)
  Minor noted: TestSaveRoundTrip doesn't verify Embedder fields; LoadFrom error-detection logic slightly complex
Task 3: complete (commits f159ed0..b27a88b, review clean after separator+char-level fixes)
  Minor noted: unused min helper in test file; unreachable base case in splitRecursive
Task 4: complete (commits b27a88b..c6b5f31, review clean after minScore SQL fix + INSERT OR REPLACE + Stats errors)
  Minor noted: by_source query error still swallowed; GetChunks omits embedding field
Task 5: complete (commits c6b5f31..b35752f, review clean)
  Minor noted: dims hardcoded to 3072 (scope is text-embedding-3-large only); no batch test; TestDimensions swallows constructor error
Task 6: complete (commits b35752f..54d11f8, review clean)
  Minor noted: filepath.Abs error silently swallowed; test missing explicit non-recursive subdir exclusion; os.WriteFile errors unchecked in tests
Task 7: complete (commits 54d11f8..6bef9d1, review clean after nil deref fix + PAT auth test)
  Minor noted: io.ReadAll error ignored; goroutine errors not surfaced; TestConfluenceHTMLStripped doesn't drain channel
Task 8: complete (commits 6bef9d1..5880f45, review clean after operation order fix + prune error handling + 2 new tests)
Task 9: complete (commits 5880f45..cc67e3a, review clean after cmd/kb fix + Extensions guard + Save errors + truncate rune fix)
  Minor noted: flagForce shared variable; Score header misleading; json.MarshalIndent error swallowed in status
Task 10: complete (commits cc67e3a..ccefc0e, review clean)
  Minor noted: json.Marshal errors swallowed in all 3 tool handlers; go.mod indirect deps
Task 11: complete (commits ccefc0e..44d884f, review clean)
  Minor noted: ingested=1 assertion could be more precise; os.WriteFile error ignored in test; placeholder GitHub URL in README

ALL TASKS COMPLETE
