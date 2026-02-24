# LLM Profiles Reference

This document lists the LLM profiles used in PhileasGo. Use this as a guide for selecting appropriate models based on their technical requirements.

## Core Profiles

| Profile | Frequency | Creativity | Output Format | Context Load | Latency Sensitive? | Special Requirements |
|:--- |:--- |:--- |:--- |:--- |:--- |:--- |
| `narration` | **Many** | High | Prose | **Large** (Wiki) | Yes (Blocking) | Creative writing, style, context handling, based on provided data. |
| `essay` | Few | **Very High** | Prose | Medium | No (Deferred) | Reasoning, broad knowledge, research capabilities, creativity. |
| `screenshot` | Few | High | Prose | Medium | Yes | **Vision (Image-to-Text)** |
| `announcements` | Medium | Medium | Prose | Small | Yes | Clear, concise speech. |
| `debriefing` | Few | Medium | Prose | **Large** (Session) | No | Summarizing flight history, discovering themes, based on provided data. |
| `border` | Few | Medium | Prose | Small | Yes | Local geography knowledge. |

## Utility & Logic Profiles

| Profile | Frequency | Creativity | Output Format | Context Load | Latency Sensitive? | Special Requirements |
|:--- |:--- |:--- |:--- |:--- |:--- |:--- |
| `script_rescue` | Medium | Low | Prose | Medium | Yes (Blocking) | Instruction Following. |
| `summary` | **Many** | Low | Prose (Short) | Medium | No | Concise event logging. |
| `thumbnails` | Many | Low | **JSON / Text ID** | Small | No | Wikipedia knowledge, smart filtering. |
| `pregrounding` | Medium | Low | Prose (Clean) | Small | No | Search / RAG capability, trivia knowledge. |
| `regional_categories_ontological` | Few | Low | **Structured JSON** | Small | No | Taxonomy knowledge, strict JSON schema. |
| `regional_categories_topographical` | Few | Low | **Structured JSON** | Small | No | Geography knowledge, strict JSON schema. |

## Profile Descriptions

### `narration` (POI)
The main voice of Phileas. It receives a full Wikipedia article snapshot and must synthesize an interesting narration, possibly in a specific literary style. This is the most demanding profile in terms of "character" and context handling.

### `essay`
Used for deep-dives into regional history or culture. It doesn't block the flight loop and can handle more complex reasoning, but requires high creativity to keep the tour interesting.

### `screenshot`
Triggered when the user takes a screenshot. Requires an LLM with **Vision** capabilities to describe the scene in the context of the current flight location.

### `announcements`
Used for short, clear spoken alerts triggered by flight events (e.g., border crossings). Requires an LLM that can be concise and articulate.

### `debriefing`
Generated at the end of a flight. It analyzes the entire session's trip log to discover themes, summarize accomplishments, and provide a Victorian-style retrospective of the journey.

### `border`
Triggered when the aircraft crosses a national border. It provides localized geography knowledge and historical context specific to the border region being traversed.

### `script_rescue` & `summary`
These are logic tasks. `script_rescue` is invoked if the primary model produces too many tokens or fails formatting; it needs to be highly reliable at stripping markdown and following word counts. `summary` creates short event logs for the session history.

### `thumbnails`
A utility profile that sifts through Wikipedia image candidates. It uses smart filtering to select the most representative image for a POI based on its category and article context.

### `pregrounding`
Enriches the context for a POI by performing preliminary data gathering. It often uses search or RAG capabilities to find "trivia" or specific facts that aren't present in the basic Wikipedia summary.

### `regional_categories_*`
Highly technical profiles that suggest new Wikidata categories to watch for based on current coordinates. These **must** return valid, structured JSON.

## Configuration Example

Here is how you would configure a hypothetical provider **XYZ** in `phileas.yaml` using three different model tiers:

```yaml
llm:
  providers:
    xyz:
      enabled: true
      profiles:
        # High-creativity / Demanding tasks
        narration: "xyz-expensive"
        essay: "xyz-expensive"

        # Vision-specific / Tool-ready tasks
        screenshot: "xyz-tools"
        thumbnails: "xyz-tools"
        pregrounding: "xyz-tools"

        # Fast / Utility / High-frequency tasks
        announcements: "xyz-fast"
        debriefing: "xyz-fast"
        border: "xyz-fast"
        script_rescue: "xyz-fast"
        summary: "xyz-fast"
        regional_categories_ontological: "xyz-fast"
        regional_categories_topographical: "xyz-fast"
```
