# Prompt Templates

This directory contains Go text templates for generating LLM prompts.

## Directory Structure

```
prompts/
├── common/          # Shared template definitions (macros)
│   └── macros.tmpl  # Defines {{template "persona"}} and {{template "style"}}
├── category/        # Category-specific instructions
│   └── aerodrome.tmpl
├── narrator/        # Main narration templates
│   ├── script.tmpl  # POI narration prompt
│   └── essay.tmpl   # Regional essay prompt
├── units/           # Unit system instructions
│   ├── imperial.tmpl
│   ├── metric.tmpl
│   └── hybrid.tmpl
└── README.md        # This file
```

## Data Fields

Available fields in narrator templates (accessed via `{{.FieldName}}`):

### Tour Guide
| Field | Type | Description |
|-------|------|-------------|
| `TourGuideName` | string | Name of the tour guide (e.g., "Ava") |
| `Persona` | string | Personality description |
| `Accent` | string | Voice accent style |

### POI Information
| Field | Type | Description |
|-------|------|-------------|
| `POINameNative` | string | POI name in local language |
| `POINameUser` | string | POI display name for user |
| `Category` | string | POI category (e.g., "Aerodrome", "Mountain") |
| `WikipediaText` | string | Wikipedia article extract |

### Location & Navigation
| Field | Type | Description |
|-------|------|-------------|
| `Country` | string | Country code (e.g., "US", "DE") |
| `Region` | string | Region/city name |
| `NavInstruction` | string | Direction/distance instruction |
| `Lat` | float64 | Current latitude |
| `Lon` | float64 | Current longitude |

### Flight Context
| Field | Type | Description |
|-------|------|-------------|
| `FlightStage` | string | Current flight phase (taxi/takeoff/cruise/descent/landing) |
| `AltitudeMSL` | float64 | Altitude above mean sea level (feet) |
| `AltitudeAGL` | float64 | Altitude above ground level (feet) |
| `Heading` | float64 | Aircraft magnetic heading (degrees) |
| `GroundSpeed` | float64 | Ground speed (knots) |
| `PredictedLat`| float64 | Predicted latitude (for nav calculation) |
| `PredictedLon`| float64 | Predicted longitude (for nav calculation) |
| `RecentContext` | string | Recently narrated POIs (to avoid repetition) |

### Settings
| Field | Type | Description |
|-------|------|-------------|
| `Language` | string | Target language code (e.g., "en-US") |
| `MaxWords` | int | Maximum narration length |
| `UnitsInstruction` | string | Rendered units template (imperial/metric/hybrid) |
| `TTSInstructions` | string | TTS-specific formatting instructions |
| `Interests` | []string | User interest topics (use with `interests` function) |

## Template Functions

### Built-in
- `{{template "name" .}}` - Include a shared template (e.g., `persona`, `style`)
- `{{.FieldName}}` - Insert data field

### Custom Functions

#### `interests` - Randomized topic list
Shuffles interests and randomly excludes 2 topics (if ≥4) for variety.
```
{{interests .Interests}}
→ "Architecture, Pop Culture, History"  (varies each render)
```

#### `category` - Category-specific instructions
Loads category template if it exists (e.g., `category/aerodrome.tmpl`).
```
{{category .Category .}}
```

#### `maybe` - Conditional inclusion (probabilistic)
Includes content with N% probability. **Re-rolls each render.**
```
{{maybe 50 "This appears 50% of the time."}}
```

#### `pick` - Random selection
Picks one option from a `|||` separated list. **Re-rolls each render.**
```
{{pick "Option A|||Option B|||Option C"}}
→ One of the three options, randomly selected
```

> [!WARNING]
> **No nested templates!** You cannot use `{{.Field}}` syntax inside function arguments.
> ```
> ❌ {{maybe 50 "Text with {{.Name}} inside"}}
> ✅ {{maybe 50 "Text without template variables"}}
> ```

## Variety Mechanisms

The prompt system uses several techniques to create varied narrations:

| Mechanism | Where | Behavior |
|-----------|-------|----------|
| **Temperature** | LLM config | Bell curve around `base` ± `jitter` (e.g., 1.0 ± 0.2) |
| **`interests`** | `script.tmpl` | Shuffles list, excludes 2 random topics |
| **`maybe N`** | Any template | Includes content with N% probability |
| **`pick`** | Any template | Randomly selects one option from variants |
| **Max words** | Config | Varies between `max_words_min` and `max_words_max` |

## Configuration

In `phileas.yaml`:
```yaml
narrator:
    max_words: 500
    max_words_min: 400
    max_words_max: 600
    temperature_base: 1.0
    temperature_jitter: 0.2  # Bell curve: most likely 1.0, range [0.8, 1.2]
```

## Examples

### Using `maybe` for optional instructions
```
{{maybe 30 "- Reference a sensory detail (sound, smell, texture)."}}
```

### Using `pick` for varied endings
```
{{pick "End on a cliffhanger.|||Leave them curious.|||End with a question."}}
```

### Combining both
```
{{maybe 50 (pick "Be extra enthusiastic!|||Add a touch of humor.")}}
```
