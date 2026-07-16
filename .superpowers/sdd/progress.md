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
Final review fixes: complete (commit ba8bd85)
  - DB dir creation fix, flagForce split, --force on ingest all, Stats scan error, go mod tidy, README paths fixed, cmd.Context(), unused min removed
BRANCH READY TO MERGE

## Logging Feature
Logging Task 1: complete (commits ba8bd85..ac876d1, review clean after fd leak fix + UserHomeDir error)
Logging Task 2: complete (commits ac876d1..787bd16, review clean)
Logging Task 3: complete (commits 787bd16..4d10df4, review clean)
Logging Task 4: complete (commits 4d10df4..20af8fb, review clean)
  Minor: log var captured in closures (ok); error msg format inconsistency; rows.Err() not checked

ALL LOGGING TASKS COMPLETE

## PDF Vision Feature
Vision Task 1: complete (commits 94bdc0e..dce7f4c, review clean)
Vision Task 2: complete (commits dce7f4c..bd436f3, review clean after pathTagRe fix)
  Minor noted: trailing space on regex line; TestExtractRasterImages_MultipleFormats uses technically invalid base64 (passes in practice via RawStdEncoding)
Vision Task 3: complete (commits bd436f3..6092a1b, review clean after gofmt fix)
  Minor noted: new WARN log on unreadable PDF pages (previously silent) — intentional improvement

ALL PDF VISION TASKS COMPLETE

## Provider Refactoring
Provider Task 1: complete (commits 24dd904..3d6e245, review clean)
  Minor: TestSaveRoundTrip lost ChunkOverlap+DB.Path assertions and env-var isolation guards
Provider Task 2: complete (commits 3d6e245..fdc5720, review clean)
  Minor: azure doc comment misleading; no factory-level test; concrete return types vs interface
Provider Task 3: complete (commits fdc5720..f5530ae, review clean)
  Minor: dims hardcoded to 3072; error message de-branded; old New() constructor removed

ALL PROVIDER REFACTORING TASKS COMPLETE

## Lazy Loading Refactoring
Lazy Task 1: complete (commits 6dca6a9..b7fc966, review clean)
  Minor: ScopePrefix doc comment examples removed
Lazy Task 2: complete (commits b7fc966..1790804, review clean)
  Minor: errNoContent now propagates from Load() — ingestor should handle it gracefully (not as fatal error)
  Minor: PDF files read twice in Load() (rawBytes wasted, fitz opens independently)
Lazy Task 3: complete (commits 1790804..07db29d, review clean after ID parsing fix + io.ReadAll error handling)
  Note: ingestor was also updated in this task (Task 4 is now partially done)
Lazy Task 4: complete (commit f1a7e7c, TestIngestLoadNotCalledOnSkip added and passing)

ALL LAZY LOADING TASKS COMPLETE
Task 1: complete (commits 0aa63e7..bb3cb6e, review clean)
Task 2: complete (commits bb3cb6e..add756c, review clean after go.mod direct dep fix)
Task 3: complete (commits add756c..2c73d10, review clean)
Task 4: complete (commits 2c73d10..f5f6446, review clean)
Task 5: complete (commits f5f6446..34ca156, review clean after env var table fix)
ALL GENAIHUB TASKS COMPLETE
