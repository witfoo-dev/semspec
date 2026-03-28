# Bug: Gemini rejects tool list — duplicate submit_work declaration

**Severity**: Blocker for Gemini execution phase
**Component**: Tool list assembly (agentic-tools or prompt assembly)
**Found during**: UI E2E T2 @easy with Gemini (2026-03-27)

## Summary

Gemini API returns 400 Bad Request: `Duplicate function declaration found: submit_work`.
All decomposer loops fail immediately with `model_error`. Planning phase works fine
(planner, requirement-generator, scenario-generator don't use execution tools).

## Evidence

```
ERROR msg="Failed to complete chat" model=gemini-flash
  error="400 Bad Request: Duplicate function declaration found: submit_work"

INFO msg="Loop failed" reason=model_error iterations=0
```

All 6 requirement decomposer loops failed with this error. Zero tokens burned on
execution — the API rejects the request before any inference.

## Root Cause

The tool list sent to Gemini includes `submit_work` twice. Likely sources:
1. `terminal.Executor.ListTools()` returns both `submit_work` and `submit_review`
2. `agentictools.RegisterTool("submit_work", termExec)` registers it again
3. The tool assembly merges both sources without deduplication

OpenAI's API silently accepts duplicate function names. Gemini's API is strict
and rejects them.

## Planning Phase (works)

Gemini Pro + Flash handled the full planning cascade:
- Goal synthesis (Gemini Pro)
- 6 requirements generated (Gemini Flash)
- 16 scenarios generated (Gemini Flash)
- Plan reviewed + scenarios reviewed
- Total cascade time: ~60s

## Fix

Deduplicate the tool list before sending to the LLM. Either:
1. Use a Set/map when assembling tool definitions
2. Or ensure `ListTools()` and `RegisterTool()` don't double-register
