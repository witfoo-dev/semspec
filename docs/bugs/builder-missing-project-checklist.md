# Bug: Builder prompt has hardcoded checklist, not project-specific checks

**Severity**: High — causes unnecessary validation failures and builder retries
**Component**: `execution-manager`, `prompt/domain/software.go`
**Found during**: UI E2E T2 @easy with Gemini (2026-03-28)
**Status**: FIXED

## Summary

The builder prompt injects a hardcoded generic structural checklist:

```
STRUCTURAL CHECKLIST — You will be auto-rejected if ANY item fails:
- No hardcoded API keys, passwords, or secrets in source code
- All errors must be handled or explicitly propagated
- No debug prints, TODO hacks, or commented-out code in the submission
- Do NOT modify files outside the declared file scope
```

But the structural-validator runs the actual project-specific checks from
`.semspec/checklist.json`:

```json
[
  {"name": "go-build", "command": "go build ./...", "description": "Compile all Go packages"},
  {"name": "go-vet",   "command": "go vet ./...",   "description": "Run Go static analysis"},
  {"name": "go-test",  "command": "go test ./...",  "description": "Run Go test suite"}
]
```

The builder has no idea that `go build ./...` and `go vet ./...` will run. It writes
code that satisfies the generic checklist but fails compilation, causing retry loops.

## Impact

In E2E run 3: structural validation passed 3/14 times. Builders kept writing code
with import errors, missing dependencies, etc. — things they would have caught if
they knew `go build` was a validation check.

## Expected Behavior

1. `execution-manager.buildAssemblyContext()` should load the project checklist and
   pass it via `TaskContext`
2. The builder prompt fragment should inject the actual check names, commands, and
   descriptions instead of the hardcoded list
3. Tester prompt should also see the checklist — knowing `go test ./...` runs helps
   them write tests in the right location and format

## Proposed Changes

### Backend (execution-manager)
- Add `Checklist []ChecklistItem` field to `prompt.TaskContext`
- In `buildAssemblyContext()`, load checklist from sandbox (or cache from config)
  and populate the field

### Prompt (software.go)
- Builder structural checklist fragment: if `ctx.TaskContext.Checklist` is non-empty,
  format the actual checks; otherwise fall back to the generic list
- Format: `- go-build: go build ./... (Compile all Go packages) [required]`

## Files

- `prompt/context.go` — add Checklist field to TaskContext
- `prompt/domain/software.go:253` — builder checklist fragment (line ~253)
- `prompt/domain/software.go:109` — developer checklist fragment (line ~109)
- `processor/execution-manager/component.go:1629` — buildAssemblyContext (populate checklist)
- `processor/structural-validator/executor.go:262` — loadChecklist (reference impl)
