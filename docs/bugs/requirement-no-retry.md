# Bug: Failed requirements are terminal — no retry mechanism

**Severity**: High — causes premature plan failure
**Component**: `requirement-executor` (`processor/requirement-executor/component.go`)
**Found during**: UI E2E T2 @easy with Gemini (2026-03-28)
**Status**: OPEN

## Summary

When a requirement's DAG node fails (task escalated, timed out, or errored), the
requirement is marked `failed` with no retry. This is terminal — the requirement
cannot be retried even if the failure was transient (API timeout, expo backoff
exhaustion, flaky validation).

## Root Cause

`requirement-executor.markFailedLocked()` (line ~1036) sets `terminated = true`
and publishes a `RequirementExecutionCompleteEvent` with `outcome: "failed"`. There
is no retry loop at the requirement level.

The TDD pipeline (execution-manager) has retries within a task node — validation
failure retries the builder, review rejection retries builder or tester. But if the
entire node fails (e.g., all TDD iterations exhausted → escalated), the requirement
is immediately terminal.

## Impact

- A single transient failure in one DAG node kills the entire requirement
- With 7 requirements for a simple /health endpoint, even a 50% node success rate
  means most requirements fail
- Plan reaches `rejected` even when some requirements completed successfully
- No way to retry just the failed requirements without re-running the entire plan

## Expected Behavior

When a requirement's node fails:
1. Check if retries remain at the requirement level (separate from task-level max_iterations)
2. If retriable: reset the failed node and re-dispatch (preserving completed nodes in the DAG)
3. If not retriable: mark requirement as failed (current behavior)
4. Pass prior work context (test output, partial code) to the retry attempt

## Workaround

Increase `execution-manager.max_iterations` to give tasks more room within a single
attempt. This reduces the chance of escalation but doesn't address the fundamental
issue of no requirement-level retry.

## Files

- `processor/requirement-executor/component.go:1036-1062` — `markFailedLocked()` (terminal, no retry)
- `processor/execution-manager/component.go:1177-1246` — `handleRejectionLocked()` (task-level retry exists)
- `processor/execution-manager/config.go:40-43` — `MaxIterations` (shared TDD budget)
