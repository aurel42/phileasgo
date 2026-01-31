# Refactoring Status Report

Generated at: 2026-01-31

## Overview
This document summarizes the current state of the application refactoring relative to the last commit. The primary objective was to decouple the announcement system and migrate screenshot handling.

## 1. Feature: Screenshot Migration
**Status**: [PARTIALLY COMPLETE]
- [x] **New Component**: `pkg/announcement/screenshot.go` created to handle screenshot events via `watcher`.
- [x] **Event Handling**: `handler_ai_queue.go` updated to inject `ImagePath` into generation requests.
- [/] **Wiring**: `main.go` updated to initialize `watcher` and pass it to `AIService`.
    - *Issue*: `NewAIService` signature in `main.go` was updated but had issues (extra `nil` argument).
- [ ] **Cleanup**: Legacy `CheckScreenshots` method in `NarrationJob` removed, but tests (`narration_job_test.go`) are currently broken because they still reference it or use the old constructor.

## 2. Refactor: Announcement System
**Status**: [IN PROGRESS]
- [/] **Decoupling**: `AnnouncementManager` is intended to run independently.
- [x] **New Files**: `pkg/announcement/border.go` (untracked) created.
- [!] **Integration**: Wired into `main.go` but causing signature mismatches in `setupScheduler`.

## 3. Refactor: Scheduler & Heartbeaters
**Status**: [PENDING REVERT]
- [x] **Implementation**: Added `Heartbeater` interface to `pkg/core/interfaces.go` and updated `Scheduler` to iterate them.
- [!] **Design Flaw**: User identified that `Heartbeater` is redundant given the existing `TimeJob` infrastructure.
- **Next Action**: **Convert `AnnouncementManager` and `AIService` persistence to standard `TimeJob`s** running at 1Hz on the existing scheduler loop. Remove `Heartbeater` interface.

## 4. Compilation & Test State
**Status**: [BROKEN]
- **Main Application**: `main.go` fails compilation due to `NewAIService` and `NewNarrationJob` argument mismatches.
- **Tests**:
    - `pkg/core/narration_job_test.go`: Fails due to `NewNarrationJob` signature mismatch and usage of removed `CheckScreenshots` method.
    - `pkg/core/narration_frequency_test.go`: Fails due to `NewNarrationJob` signature mismatch.
    - `pkg/core/scheduler_test.go`: Semantic mismatch with `NewScheduler` args.

## Plan to Fix
1.  **Revert/Remove Heartbeater**: Delete `Heartbeater` interface and remove from `Scheduler`.
2.  **Implement TimeJobs**: In `setupScheduler` (main.go), wrap `annMgr.Tick` and `narratorSvc.Persist` in `core.NewTimeJob` (1Hz).
3.  **Fix Constructor Signatures**:
    - Align `NewNarrationJob` in all tests to the new signature (no watcher).
    - Align `NewAIService` call in `main.go` to the definition.
4.  **Verify**: Run `go vet ./...` until clear.
