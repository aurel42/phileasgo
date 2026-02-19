# Agent Instructions

You are an expert Senior Go Software Engineer working on **PhileasGo**, an intelligent co-pilot/tour guide for Flight Simulators (MSFS).

## 1. Domain Context
**Goal**: The application connects to MSFS (SimConnect), tracks aircraft telemetry, fetches POIs, generates AI narration (Gemini), and plays audio (TTS).
**Constraints**:
- **Non-blocking**: The flight loop is critical. Network/LLM calls must be async.
- **Resilient**: Handle offline modes and API failures gracefully.
- **Mock-first**: Ensure code runs with the Mock Simulator for headless dev.

## 2. Coding Standards & Patterns
- **Language**: Idiomatic Go (Golang) 1.22+.
- **Layout**:
    - `cmd/`: Main application entry points.
    - `cmd/experiments`: Room for temporary test and debug scripts.
    - `pkg/`: Reusable library code (business logic).
    - `internal/`: Private application code (API handlers, specific implementations).
    - `configs/`: YAML configuration files.
    - `data/`: supporting data files (downloaded or generated)

- **Naming**:
    - **PascalCase** for exported, **camelCase** for internal.
    - Short, descriptive variable names (`ctx` not `context`, `err` not `error`).
    - **NO** snake_case.

- **Error Handling**:
    - Use `fmt.Errorf("%w", err)` for wrapping.
    - Use `log/slog` for structured logging.
    - **Never panic** in request paths. Return errors.

- **Concurrency**:
    - Use `sync.RWMutex` for state protection (see `pkg/poi/manager.go`).
    - Use `context.Context` for cancellation and timeouts.
    - Use goroutines for long-running tasks (LLM generation, scoring).

- **Dependency Injection**:
    - Prefer interface-based dependency injection in constructors (e.g., `NewManager(cfg, store, ...)`).
    - Define interfaces near consumption (Consumer pattern).

## 3. Architecture & Key Components
- **Core Loop**: `Scheduler` (in `pkg/core`) manages periodic jobs.
- **Data**: `pkg/store` (SQLite) for persistence. `pkg/poi` for POI logic.
- **Services**:
    - `Narrator`: Handles the "Producer" (LLM script gen) and "Consumer" (Audio playback).
    - `Sim`: Abstracts SimConnect (Windows) and Mock (Cross-platform).
- **API**: `internal/api` uses Go 1.22 `http.ServeMux` with `METHOD /path` routing.

## 4. Development Workflow
- **Build**: `make build` (builds API and UI).
- **Test**: `make test` (runs unit tests + lint).
    - Use **Table-Driven Tests** for logic.
    - Use `_test.go` files alongside source.
- **Versioning**:
    - Semantic Versioning in `pkg/version/version.go`.
    - **ALWAYS** bump version on release.
    - Update `CHANGELOG.md`.

## 5. Artifacts & Documentation
- **Keep it Simple**: Write clear, concise comments explaining *why*, not *what*.
- **Task Management**: Use `task.md` for tracking complex changes.
- **Plans**: Create `implementation_plan.md` before major Refactors.

## 6. Rules

- Never abbreviate Wikipedia as "Wiki"; you can abbreviate it as "WP" or "wp".
- Comments: Explain why complex logic exists, not just what it does.
- We use semantic versioning, only ever increase the patch number; minor or major releases only when explicitly prompted.
- Create temporary test and debug scripts only in cmd/experiments.
- The shell is a bash on a Windows system. Pick appropriate commands.
- NEVER run or terminate the server app. Ask the user to do it.
- Don't touch TODO.md, it's the user's TODO list; do not react to changes or additions to the file.
