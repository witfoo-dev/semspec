# Bug: Gemini rejects tool list — duplicate submit_work declaration

**Severity**: Blocker for Gemini execution phase
**Component**: Tool registration (`tools/register.go`)
**Found during**: UI E2E T2 @easy with Gemini (2026-03-27)
**Status**: FIXED

## Summary

Gemini API returns 400 Bad Request: `Duplicate function declaration found: submit_work`.
All decomposer loops fail immediately with `model_error`.

## Root Cause

`terminal.Executor` handles both `submit_work` and `submit_review`. When registered
under two names with the same executor instance, `ListRegisteredTools()` calls
`executor.ListTools()` on each entry — producing `[submit_work, submit_review,
submit_work, submit_review]`. OpenAI silently accepts duplicates; Gemini rejects them.

## Fix

Added `singleToolAdapter` wrapper in `tools/register.go` that filters `ListTools()`
to return only the definition matching the registered name. Each registration now
wraps the shared executor:

```go
termExec := terminal.NewExecutor()
_ = agentictools.RegisterTool("submit_work", singleToolAdapter(termExec, "submit_work"))
_ = agentictools.RegisterTool("submit_review", singleToolAdapter(termExec, "submit_review"))
```

This produces a clean tool list: `[submit_work, submit_review]` — no duplicates.
