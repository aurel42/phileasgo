# PhileasGo

AI-powered tour guide companion for Microsoft Flight Simulator.

PhileasGo narrates points of interest as you fly, providing contextual information about landmarks, cities, and geographic features using AI-generated descriptions.

## Features

- **Real-Time AI Narration**: Phileas acts as your tour guide, identifying landmarks as you fly and generating context-aware, engaging spoken tours in real-time using Google Gemini.
- **Smart Data Ingestion**: Pulls data from Wikidata and Wikipedia, using optimized SparQL queries and heavy caching (SQLite) to be kind to the APIs while ensuring you always have fresh data.
- **Complex Scorer Logic**: Points of Interest aren't just "nearest". The scorer weighs distance, bearing, category, and novelty to pick the most interesting topic.
- **Variety Engine**: Extensive logic dedicated to keeping the narration fresh. The tour guide tracks what has been said to ensure new topics and different angles are explored.
- **Robust TTS Support**:
    - **Edge TTS**: Zero-config, high-quality neural voices (free).
    - **Azure Speech**: Premium neural voices (requires subscription, free tier available).
    - **Fish Audio**: Experimental support for highly emotive, next-gen AI voices (paid service).
    - **Local SAPI**: Fallback to Windows native TTS.
- **In-Sim Markers**: Spawns visual markers (balloons) in the simulator so you can instantly spot the POI being discussed. No more guessing which mountain the guide is talking about.
- **1000cities Integration**: Automatically detects which country you are flying over to ensure correct localized pronunciation of names and appropriate context.
- **Dynamic Configuration**: Occasionally consults an LLM to "learn" what types of POIs are culturally or geographically significant for your current region, adjusting the scorer weights on the fly.
- **Flight Sim Focused**: 
    - **SimConnect Integration**: Seamlessly reads telemetry from MSFS.
    - **Built-in Mock Sim**: Includes a simulator loop for testing without launching the sim.
    - **Resource Efficient**: Written in Go to be lightweight (~100MB RAM), leaving your system resources for the sim.
- **Highly Configurable**: Nearly every aspect of the logic—from prompt macros to scoring weights—can be tweaked via YAML configuration or prompt templates.
- **Resilient**: Designed to handle real-world conditions: high/low density areas, slow API responses, and long-haul flights without crashing or leaking memory.

## Requirements

- Windows 10/11
- Microsoft Flight Simulator 2024
- Google Gemini API key (**required**)
- Azure TTS credentials (optional - Edge TTS works without configuration)

## Limitations

- Support for other LLMs is missing. At the time of the creation of Phileas, I only had access to Gemini. The LLM is used to create a script for the tour guide based on the information from Wikipedia and to add regional categories on the fly. The bulk of the requests can be handled by a cheap LLM model like gemini-2.5-flash-lite, resulting in negligible cost (even with extensive testing, I'm billed about 20 cents/day; I also tested gemini-pro-latest and, for maybe marginally better results, I got billed several Euros/day).
- Support for MSFS2020 is missing. I only tested MSFS2024, and I'm pretty sure the balloons have a different name in MSFS2020, so the markers wouldn't work. Should be an easy fix if someone is interested.

## Installation

1. Download the latest release
2. Extract to a folder of your choice
3. Run the installation helper in PowerShell:
   ```powershell
   .\install.ps1
   ```
   (alternatively, right-click install.ps1 and select "Run with PowerShell")
4. Edit `configs/phileas.yaml` and add your Gemini API key

The install script will:
- Download GeoNames city data
- Prompt you to download MSFS POI data
- Generate the default configuration file

## Configuration

Edit `configs/phileas.yaml` to configure API keys:

```yaml
llm:
  gemini_key: "YOUR_GEMINI_API_KEY"  # Required

tts:
  provider: "edge"  # Default, no additional config needed
  # Or use Azure for higher quality:
  # provider: "azure"
  # azure_key: "YOUR_AZURE_KEY"
  # azure_region: "eastus"
```

## Usage

1. Start Microsoft Flight Simulator
2. Run `phileasgo.exe`
3. Open http://localhost:1920 in your browser
4. Start a flight and enjoy the narration!

The web UI shows:
- Current flight telemetry
- Nearby points of interest
- Narration status and controls

## Building from Source

Prerequisites:
- Go 1.21+
- Node.js 18+
- npm
- **MSFS SDK** (required solely for SimConnect.dll)

```bash
# Install Go dependencies
go mod download

# Build everything (web + Go binary)
make build

# Or build components separately:
make build-web   # Build web UI
make build-app   # Build Go binary

# Run tests
make test

# Create vendor directory
make vendor
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

## License

MIT License - see [LICENSE](LICENSE) file for details.
