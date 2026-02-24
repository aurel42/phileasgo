---
name: phileasgo-llm-provider
description: >
  Step-by-step guide for adding a new LLM provider to PhileasGo. Use when
  setting up a new AI inference backend — whether OpenAI-compatible (Groq,
  DeepSeek, OpenRouter, local Ollama, etc.) or custom-SDK (like Gemini).
  Covers: creating the provider package, wiring into the failover chain,
  config & env setup, request-client tracker normalization, and YAML defaults.
---

# Adding an LLM Provider to PhileasGo

## Overview

LLM providers live in `pkg/llm/<name>/`. Each implements `llm.Provider`
(defined in `pkg/llm/provider.go`). Providers are instantiated in the factory
(`pkg/narrator/factory.go`), wrapped in the failover chain, and configured
via `phileas.yaml` + `.env`.

Two provider archetypes exist:

| Archetype | Example | Client code |
|---|---|---|
| **OpenAI-compatible** | Groq, DeepSeek | Thin wrapper around `pkg/llm/openai.NewClient` |
| **Custom SDK** | Gemini | Full client implementing `llm.Provider` directly |

## Step-by-step Workflow

### 1. Create the provider package

#### OpenAI-compatible (most providers)

Create `pkg/llm/<name>/provider.go`:

```go
package <name>

import (
    "phileasgo/pkg/config"
    "phileasgo/pkg/llm/openai"
    "phileasgo/pkg/request"
)

const baseURL = "https://api.<name>.com/v1"

// NewClient creates a new <Name> client using the generic OpenAI provider.
func NewClient(cfg config.ProviderConfig, rc *request.Client) (*openai.Client, error) {
    return openai.NewClient(cfg, baseURL, rc)
}
```

The `openai.Client` already implements all of `llm.Provider` (text, JSON,
image, validation, profiles). If the provider's base URL differs from the
default, set `cfg.BaseURL` in YAML and the openai client picks it up
automatically.

#### Custom SDK (non-OpenAI-compatible)

Create `pkg/llm/<name>/client.go` implementing `llm.Provider` directly.
Must implement all five methods:

```go
type Provider interface {
    GenerateText(ctx context.Context, name, prompt string) (string, error)
    GenerateJSON(ctx context.Context, name, prompt string, target any) error
    GenerateImageText(ctx context.Context, name, prompt, imagePath string) (string, error)
    ValidateModels(ctx context.Context) error
    HasProfile(name string) bool
}
```

Key patterns from the Gemini reference implementation (`pkg/llm/gemini/`):

- Constructor takes `(cfg config.ProviderConfig, rc *request.Client, t *tracker.Tracker)`.
- Store `cfg.Profiles` (map[string]string) for intent-to-model resolution.
- Track success/failure via `t.TrackAPISuccess("<name>")` / `t.TrackAPIFailure("<name>")`.
  The tracker name **must match the YAML provider key** (see step 5).
- `GenerateJSON`: use `llm.CleanJSONBlock()` before unmarshalling.
- `GenerateImageText`: use `imageutil.PrepareForLLM()` for image preprocessing.
- If the provider doesn't support images, return `fmt.Errorf("<name> does not support image input")`.
- `ValidateModels`: skip with `slog.Debug(...)` + `return nil` if the
  provider has no `/models` endpoint.

### 2. Wire into the factory

Edit `pkg/narrator/factory.go`:

1. Add the import:
   ```go
   "phileasgo/pkg/llm/<name>"
   ```

2. Add a case in the `switch pCfg.Type` block inside `NewLLMProvider`:
   ```go
   case "<name>":
       sub, err = <name>.NewClient(pCfg, rc)
   ```
   For custom-SDK providers that need the tracker:
   ```go
   case "<name>":
       sub, err = <name>.NewClient(pCfg, rc, t)
   ```

The factory already handles timeout, free-tier tracking, and failover
wrapping — no other changes needed there.

### 3. Register the env-var key loader

Edit `pkg/config/config.go`, function `loadSecretsFromEnv`.
Add a case to the `switch p.Type` block:

```go
case "<name>":
    if key := os.Getenv("<UPPER_NAME>_API_KEY"); key != "" {
        p.Key = key
    }
```

Convention: env var is `<UPPER_NAME>_API_KEY` (e.g. `GROQ_API_KEY`,
`DEEPSEEK_API_KEY`, `OPENAI_API_KEY`).

### 4. Add an entry to `.env.template`

Add the new key under the `# LLM Providers` section:

```
<UPPER_NAME>_API_KEY=
```

Place it with the other LLM keys (lines 1-4), or under the
`# specialized LLM providers` comment if it serves a niche role
(like Perplexity for grounding).

### 5. Normalize the provider name in the request client

Edit `pkg/request/client.go`, function `normalizeProvider`.
Add a mapping **before** the `return host` fallback:

```go
if strings.HasSuffix(host, "<domain>") {
    return "<name>"
}
```

The returned string **must match the YAML provider key** used in
`phileas.yaml` (step 6). This ensures the request client's per-host
serialization queue and the tracker stats both use the same name.

Existing mappings for reference:
- `googleapis.com` -> `"gemini"`
- `groq.com` -> `"groq"`
- `perplexity.ai` -> `"perplexity"`
- `deepseek.com` -> `"deepseek"`

### 6. Add a default config entry to `phileas.yaml`

Edit `pkg/config/config.go`, function `DefaultConfig`, inside the
`LLM.Providers` map. Add a sensible default (profiles can be empty or
populated with recommended models):

```go
"<name>": {
    Type: "<name>",
    Key:  "",
    Profiles: map[string]string{
        "narration": "<default-model>",
    },
    FreeTier: false,
},
```

**Do NOT add it to the `Fallback` slice in DefaultConfig** — users opt in
by adding it to their own `fallback:` list.

### 7. Update the fallback chain (user-facing config)

In the user's `configs/phileas.yaml`, add the provider name to the
`fallback` list. Order determines priority:

```yaml
llm:
  providers:
    <name>:
      type: <name>
      profiles:
        narration: "<model-id>"
        essay: "<model-id>"
  fallback:
    - gemini
    - <name>    # <-- added as secondary
```

The failover chain (in `pkg/llm/failover/`) handles everything else:
- Profile-aware routing (skips providers that don't have the requested profile)
- Circuit breaking on 401/403
- Smart exponential backoff on transient failures
- Timeout per provider (from `ProviderConfig.Timeout`, default 90s)

## Profiles

Profiles map intent names to model IDs. A provider only needs the profiles
it should handle — the failover chain skips providers that lack a requested
profile. Common profiles:

| Profile | Purpose |
|---|---|
| `narration` | POI narration scripts |
| `essay` | Long-form essay generation |
| `announcements` | Short announcements (letsgo, briefing, border) |
| `screenshot` | Image-to-text description |
| `thumbnails` | Image-to-text for thumbnails |
| `regional_categories_ontological` | Regional category classification |
| `regional_categories_topographical` | Topographical classification |
| `script_rescue` | Rescue/fallback script generation |
| `pregrounding` | Web-grounded context enrichment (Perplexity) |

## Checklist

- [ ] `pkg/llm/<name>/provider.go` — implements `llm.Provider`
- [ ] `pkg/narrator/factory.go` — new case in `switch pCfg.Type`
- [ ] `pkg/config/config.go` — `loadSecretsFromEnv` case + `DefaultConfig` entry
- [ ] `.env.template` — `<UPPER_NAME>_API_KEY=`
- [ ] `pkg/request/client.go` — `normalizeProvider` mapping
- [ ] `configs/phileas.yaml` — provider entry + `fallback` list (user action)
- [ ] Test: `go build ./...` compiles, `make test` passes
