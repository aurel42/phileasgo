# PhileasGo

AI-powered tour guide companion for Microsoft Flight Simulator.

PhileasGo narrates points of interest as you fly, providing contextual information about landmarks, cities, and geographic features using AI-generated descriptions.

## Features

- **AI Tour Guide**: Generates context-aware, real-time briefings for landmarks using LLMs. It knows where you are, your heading, and effectively "sees" the terrain.
- **Multi-Provider LLM Support**: Supports Google Gemini, Groq, and any OpenAI-compatible API (Mistral, Ollama, etc.).
- **Terrain & Visibility Awareness**: 
    - **Line-of-Sight (LOS)**: Uses ETOPO1 global elevation data to verify visibility. Phileas will rarely point out landmarks hidden behind mountain ranges.
- **Smart POI Prioritization**:
    - **Contextual Scoring**: Weighs distance, size, and novelty. Prioritizes significant landmarks (Airports, Mountains, Cities) over generic data points.
- **Immersive Audio**:
    - **Headset Effect**: Optional bandpass filter to simulate aviation headset/radio audio.
    - **Multiple Providers**: Supports Edge TTS (free/default), Azure Speech, and Fish Audio.
- **Flight Bag (Web UI)**:
    - Live map showing aircraft position, flight path, and scanned area heatmap.
    - **Manual Control**: Trigger narrations manually or review nearby POIs.
- **Visual Markers (VFR)**: Spawns in-sim balloons to help you visually identify a specific landmark.
- **Sim-Optimized**: 
    - **Lightweight**: Written in Go (~100MB RAM), runs in the background with zero impact on MSFS framerates. Vibecoded using a braindead LLM, so it's probably not that optimized, but it's good enough for me.
    - **Offline-First Design**: Heavy SQLite caching ensures that once an area is flown, it loads instantly without hammering APIs.
- **Highly configurable**: 
    - **POI Prioritization**: Configure which categories of POIs are most important to you.
    - **Narration Settings**: Adjust volume, speed, and other settings.

## Requirements

- Windows 10/11
- Microsoft Flight Simulator 2024
- An LLM API key (Gemini or Groq recommended)
- Azure TTS credentials (optional - Edge TTS works without configuration)

## LLM Providers

PhileasGo supports multiple LLM providers. You only need **one** of the following:

| Provider | Cost | Notes |
|----------|------|-------|
| **Groq** | Free tier available | Recommended for getting started. Obtaining an API key is painless—just sign up at [console.groq.com](https://console.groq.com) and create a key. The free tier is generous enough for casual use. |
| **Gemini** | Pay-per-use | Google's Gemini models. |
| **OpenAI-compatible** | Varies | Any OpenAI Chat Completions API (Mistral, Ollama, local models, proxies). |

If you have access to a paid tier from one provider, and a free tier from another provider, configure a fallback chain in configs/phileas.yaml (e.g., `llm.fallback: ["groq", "gemini"]`). Should you hit the quotas for the free tier, PhileasGo will use the next provider in the chain. This is particularly useful for Groq, which has a generous free tier but can be rate-limited during peak hours. 

Note that different models produce radically different results when it comes to the script prose, so the fallback mechanism will result in your tour guide demonstrating... erm... rhetorical range.

## Limitations

- Support for MSFS2020 is missing. I only tested MSFS2024, and I'm pretty sure the balloons have a different name in MSFS2020, so the markers wouldn't work. Should be an easy fix if someone is interested.

## Installation

1. Download the latest release
2. Extract to a folder of your choice
3. Run the installation helper in PowerShell:
   ```powershell
   .\install.ps1
   ```
   (alternatively, right-click install.ps1 and select "Run with PowerShell")
4. Copy `.env.template` to `.env.local` and add your API keys (see below)

The install script will:
- Create necessary data and log directories
- Download and extract GeoNames city data
- Download and extract ETOPO1 global elevation data (for Line-of-Sight visibility)
- Prompt you to manually download and place MSFS POI data (Master.csv)
- Generate the default configuration file if missing

### API Key Configuration

API keys are configured via environment variables in a `.env.local` file (not committed to git):

```bash
# Copy the template
cp .env.template .env.local

# Edit .env.local with your keys
```

The `.env.template` file shows all available options:

```bash
# LLM Providers (you need at least one)
GEMINI_API_KEY=your_gemini_key_here
GROQ_API_KEY=your_groq_key_here

# TTS Providers (optional - Edge TTS works without keys)
FISH_API_KEY=
SPEECH_KEY=
SPEECH_REGION=
```

> **Tip**: For the easiest setup, just get a free Groq API key from [console.groq.com](https://console.groq.com).

## Configuration

PhileasGo is designed to be highly configurable. All configuration files are located in the `configs/` directory.

### Main Configuration (`phileas.yaml`)

Edit `configs/phileas.yaml` to configure system settings:

```yaml
tts:
  engine: "edge-tts"  # Default, no additional config needed
  # options: "windows-sapi", "edge-tts", "fish-audio", "azure-speech"
```

> **Note**: API keys are configured in `.env.local`, not in `phileas.yaml`. See [API Key Configuration](#api-key-configuration) above.

### Advanced Customization

You can tweak the behavior of the tour guide by modifying other files in `configs/`:

- **`categories.yaml`**: Define which Wikidata categories are recognized.
    - Add new categories by mapping them to Wikidata QIDs.
    - Adjust `weight` to prioritize certain types of landmarks.
    - Change `size` (S/M/L/XL) to determine from how far away a landmark is visible.
    - Customize `icon` mappings for the web UI.

- **`visibility.yaml`**: Controls when Phileas "sees" a landmark.
    - Defines the maximum visibility distance (in Nautical Miles) for each size category (S/M/L/XL) at different altitudes.
    - Example: If you want "Large" landmarks to be visible from further away at 3000ft, increase the value in the table.

- **`interests.yaml`**: A list of topics that the AI uses to gauge relevance.
    - Add or remove interests to steer the "flavor" of the tour guide (e.g., add "Architecture" or "Biology" if you want the AI to focus on those aspects).

- **`essays.yaml`**: Configures the topics for longer, regional "essays".
    - When there are no immediate landmarks, Phileas talks about the region.

- **`prompts/`**: Contains the raw system prompts sent to Google Gemini.
    - **`narrator/script.tmpl`**: The main prompt used to generate landmark descriptions. You can edit this to change the personality, tone, or structure of the tour guide's narration.
    - **`category/`**: Contains dynamic sub-prompts used when flying over specific types of landmarks (e.g., `city.tmpl` instructs the LLM to search for local food and vibes, `aerodrome.tmpl` instructs the LLM to focus on aviation history).
    - **`tts/fish-audio.tmpl`**: A special prompt optimized for a specific cloned voice. It instructs the LLM to use specific mannerisms, vocabulary, and rhetorical styles that align with that persona.
    - **`common/macros.tmpl`**: Defines the shared persona traits and values. **Edit this file if the tour guide's "politics" or worldview are not to your liking.** It currently defines a narrator who is critical of imperialism/authoritarianism and sides with the oppressed; you can adjust these instructions to match your preferred narrative neutrality or bias.

## Usage

1. Start Microsoft Flight Simulator
2. Run `phileasgo.exe`
3. Open http://localhost:1920 in your browser
4. Start a flight and enjoy the narration!

The web UI shows:
- Current flight telemetry
- Nearby points of interest
- Narration status and controls

### Cockpit Control (Transponder)

PhileasGo can be controlled directly from your aircraft's transponder, allowing you to stay in the cockpit while adjusting settings.

- **Squawk-Based Settings**: Set your transponder to codes starting with `7` to change settings on the fly:
    - **Format**: `7[Freq][Len][Boost]`
    - **Digit 1 (Frequency)**: `0` (Pause), `1-5` (Narration frequency from Rarely to Constant).
    - **Digit 2 (Narrative Length)**: `1-5` (Scale text length from Shortest to Longest).
    - **Digit 3 (Visibility Boost)**: `1-5` (Scale visibility range from 1.0x to 2.0x).
    - *Example*: Squawking `7231` sets normal frequency, normal length, and no boost. `7055` pauses narration but sets max length and boost for when you resume.
- **IDENT Button**: Pressing the transponder's **IDENT** button triggers a configurable action. Use `IdentAction` in `configs/phileas.yaml` to set this to `skip` (default), `pause_toggle`, or `stop`.
    - *Note*: The IDENT trigger works regardless of your squawk code, as long as the feature is enabled.

For streaming: point the browser to http://localhost:1920/overlay for a transparent overlay. The elements can be turned on/off in `configs/phileas.yaml`.

Note: on your first flight, the Wikidata QID hierarchy cache table is populated. This will result in a high number of Wikidata API calls and is nothing to worry about. As long as your database (data/phileas.db) is present, we will rarely ask for any piece of information more than once. If you want to make it easier on Phileas, don't start your first flight in the middle of a metropolitan area.

## Building from Source

- [LittleNavMap MSFS POIs](https://flightsim.to/file/81114/littlenavmap-msfs-poi-s) - MSFS-specific landmarks
- [Uber H3](https://h3geo.org/) - Hexagonal hierarchical spatial index

## Building from Source

Prerequisites:
- Go 1.21+
- Node.js 18+
- npm
- **C Compiler (MinGW/GCC)** (required for H3 CGO)
- **MSFS SDK** (required solely for SimConnect.dll)

```bash
# Install Go dependencies
go mod download

# Run tests
make test

# Build everything (web + Go binary)
make build

# Or build components separately:
make build-web   # Build web UI
make build-app   # Build Go binary
```

## Data Sources

PhileasGo uses data from:
- [Wikidata](https://www.wikidata.org/) - Point of interest metadata
- [Wikipedia](https://www.wikipedia.org/) - Article content for narration
- [GeoNames](https://www.geonames.org/) - City and geographic data
- [LittleNavMap MSFS POIs](https://flightsim.to/file/81114/littlenavmap-msfs-poi-s) - MSFS-specific landmarks

## Project History

I started this project to see how far I could get purely by vibecoding. I always wanted something like a tour guide for MSFS, I went so far as to talk Brian of SayIntentions into adding a "tour guide" feature to his product, but I wasn't happy with the result. 

I vibecoded a series of three Python clients with growing complexity (and resource usage), and encountered the limits of current vibecoding tools (repeatedly). This fourth attempt in Go was meant to explore how a stricter language, a more structured codebase, and access to the Python proof-of-concept implementations would improve agents' abilities to manage the complexity.

It turned out so well that I'm releasing it as a public project (let's call it a "public backup"). I'm sure the code is not pretty (I don't actually code in Go, that was part of the experiment), but the resource usage is plausible and, at least for me, it appears to do what it's supposed to do.

I expect to be the only user for the foreseeable future, so at this point I don't care to put together a binary release. Let me know if you feel I should (or which missing LLM/TTS services you feel the project should support). Or if you want support for MSFS2020.

## Credits

This project uses the **[Uber H3](https://github.com/uber/h3)** geospatial indexing system.
H3 is licensed under the [Apache License 2.0](https://github.com/uber/h3/blob/master/LICENSE).

Elevation data provided by **[ETOPO1 Global Relief Model](https://www.ncei.noaa.gov/products/etopo-global-relief-model)** from NOAA National Centers for Environmental Information.
Citation: Amante, C. and B.W. Eakins, 2009. ETOPO1 1 Arc-Minute Global Relief Model: Procedures, Data Sources and Analysis. NOAA Technical Memorandum NESDIS NGDC-24. National Geophysical Data Center, NOAA. doi:10.7289/V5C8276M

Category icons provided by **[Mapbox Maki](https://github.com/mapbox/maki)** (CC0 1.0 Universal).


## License

MIT License - see [LICENSE](LICENSE) file for details.
