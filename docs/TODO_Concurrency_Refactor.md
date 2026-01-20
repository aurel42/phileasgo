# Implementation Plan - Concurrency Refactor

The goal is to strictly serialize LLM/Narrative generation while ensuring user requests (Manual POI, Screenshot) are never discarded, even if another generation is in progress.

# [Goal Description]

Refactor the narration architecture to separate **User Requests** (Priority) from **System Events** (Backfill).
1. **Priority Generation Queue**: `AIService` manages a queue specifically for User Manual POIs and Screenshots. These are strictly serialized.
2. **Orchestration**: The main scoring loop in `main.go` acts as the orchestrator. It checks the Priority Queue first. Only if empty does it consult `NarrationJob.CanPrepare*` for system events.
3. **NarrationJob Cleanup**: `ShouldFire` is removed. It is replaced by specific `CanPreparePOI` and `CanPrepareEssay` checks.

## User Review Required

> [!NOTE]
> **Orchestration Change**: The `main.go` callback loop will now look like:
> ```go
> if narrator.HasPriority() {
>     narrator.ProcessPriority()
> } else if job.CanPrepareDebrief() {
>     job.RunDebrief()
> } else if job.CanPreparePOI() {
>     job.RunPOI()
> } else if job.CanPrepareEssay() {
>     job.RunEssay()
> }
> ```
> This ensures user requests always block system generation attempts before they start.

## Proposed Changes

### pkg/model

#### [NEW] [narrative.go](file:///c:/Users/aurel/Projects/phileasgo/pkg/model/narrative.go)
- Move `Narrative` struct to `pkg/model` to avoid circular dependencies.
- Add `NarrativeType` enum/constants.
- Unify `POI`, `Essay`, `Debrief` types under this model.

### pkg/narrator

#### [MODIFY] [service_ai.go](file:///c:/Users/aurel/Projects/phileasgo/pkg/narrator/service_ai.go)
- Add `priorityGenQueue []*GenerationJob` (mutex protected).
- Add `HasPendingPriority() bool`.
- Add `ProcessPriorityQueue(ctx)`.
- Update `PlayPOI` (Manual): Enqueue `GenerationJob`.
- Update `PlayImage`: Enqueue `GenerationJob`.

#### [MODIFY] [service_ai_poi.go](file:///c:/Users/aurel/Projects/phileasgo/pkg/narrator/service_ai_poi.go)
- Refactor `PlayPOI`:
    - **Manual**: Wrap `GenerateNarrative` in a closure, add to `priorityGenQueue`.
    - **Auto**: Check `HasPendingPriority()`. If true, abort (backoff).
- Update `GenerateNarrative`: Simplify (remove force/cancel).

### pkg/core

#### [MODIFY] [narration_job.go](file:///c:/Users/aurel/Projects/phileasgo/pkg/core/narration_job.go)
- **Removed**: `ShouldFire` (deprecated).
- **Added**: Granular checks:
    - `CanPrepareDebrief(t)`
    - `CanPreparePOI(t)`
    - `CanPrepareEssay(t)`
- **Added**: Granular execution methods:
    - `RunDebrief(ctx, t)`
    - `RunPOI(ctx, t)`
    - `RunEssay(ctx, t)`
- **Logic**: 
    - `CanPreparePOI` handles frequency, cooldowns, and suppression for POIs.
    - `CanPrepareEssay` handles essay-specific constraints.
    - `Run` is deprecated/removed in favor of specific `Run*` methods.

### cmd/phileasgo

#### [MODIFY] [main.go](file:///c:/Users/aurel/Projects/phileasgo/cmd/phileasgo/main.go)
- Orchestration:
  ```go
  if narrator.HasPriority() {
      // User Request (Manual/Screenshot/Debrief)
      go narrator.ProcessPriorityQueue(ctx)
  } else if job.CanPrepareDebrief(t) {
      // System Event: Debrief
      go job.RunDebrief(ctx, t)
  } else if job.CanPreparePOI(t) { 
      // System Event: POI
      go job.RunPOI(ctx, t)
  } else if job.CanPrepareEssay(t) {
      // System Event: Essay
      go job.RunEssay(ctx, t)
  }
  ```

### cmd/phileasgo

#### [MODIFY] [main.go](file:///c:/Users/aurel/Projects/phileasgo/cmd/phileasgo/main.go)
- Update the `PoiMgr.SetScoringCallback`:
    1. Check `screenWatcher` (if enabled). If new file -> call `narrator.PlayImage` (which enqueues).
    2. Check `narrator.HasPendingPriority()`. If true -> `go narrator.ProcessPriorityQueue()`.
    3. Else -> Check `job.CanPreparePOI/Essay` -> `go job.RunPOI/Essay`.

## Verification Plan

### Automated Tests
- **Queue Priority**: `service_ai_queue_test.go`:
    1. Enqueue Manual Job.
    2. Assert `HasPendingPriority()` is true.
    3. Simulate Auto call -> Assert it returns early (backoff).
    4. Process Priority -> Assert Manual runs.

### Manual Verification
1. **Concurrency Test**: 
    - Trigger Auto POI.
    - Click Manual POI immediately.
    - Take Screenshot immediately.
    - **Expect**: Auto finishes. Manual POI runs. Screenshot runs. No cancellation.

## Learnings & Handover Notes

### 1. Resolved: TestPlayDebrief_Busy Panic
- **Issue**: `TestPlayDebrief_Busy` in `pkg/narrator` was panicking.
- **Cause**: The `AIService` constructor validates dependencies. Even though `PlayDebrief` handles logic internally, the service initialization or background queue processing might touch components (like TTS or Audio) that were passed as `nil` in the test setup. Specifically, `MockTTS` was missing.
- **Fix**: Always provide full mock implementations (`&MockTTS{}`, `&MockAudio{}`) when initializing `AIService` in tests, even if you don't expect them to be used. This prevents nil pointer dereferences in background goroutines or unexpected code paths.

### 2. Active Issue: pkg/core Test Failure
- **Test**: `TestNarrationJob_GroundSuppression/Paused:_High_Score_POI_->_No_Narration` fails.
- **Symptom**: The test expects `CanPreparePOI` to return `false` (because `isPaused` is true), but it returns `true`.
- **Logic**: `CanPreparePOI` calls `checkNarratorReady`, which checks `narrator.IsPaused()`.
- **Hypothesis**: The mock setup in `pkg/core/narration_job_test.go` might be failing to propagate the `isPaused` state correctly, or `checkNarratorReady` is being bypassed or returning valid prematurely.
- **Next Steps**: 
    - Verify `mockNarratorService` implementation in `pkg/core`.
    - Use logging/panic inside `IsPaused` mock to verify it is called.
    - Check if `CanPreparePOI` has a path that returns `true` *before* calling `checkNarratorReady`.

### 3. General Advice
- **Dependency Injection**: When refactoring `AIService` or `NarrationJob`, strictly maintain dependency injection in tests. The "split" between `CanPreparePOI`/`Essay` logic and the `PriorityQueue` logic in `main.go` creates complex interaction states that must be carefully mocked.
- **Async & Locks**: `NarrationJob` and `AIService` now use heavy locking (`RWMutex`) and background queues. Ensure tests simulate this correctly or use sufficient `time.Sleep` or channel synchronization to avoid race conditions.
