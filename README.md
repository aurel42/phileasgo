# PhileasGo

AI-powered tour guide companion for Microsoft Flight Simulator.

PhileasGo narrates points of interest as you fly, providing contextual information about landmarks, cities, and geographic features using AI-generated descriptions.

## Features

- **Real-time narration** of points of interest during flight
- **AI-generated descriptions** using Google Gemini for contextual, engaging content
- **Text-to-speech** narration (Edge TTS by default, Azure TTS optional)
- **SimConnect integration** for seamless MSFS connectivity
- **Web-based UI** accessible via browser at http://localhost:1920
- **POI beacon markers** visible in the simulator

## Requirements

- Windows 10/11
- Microsoft Flight Simulator 2020 or 2024 (Steam or Microsoft Store)
- Google Gemini API key (**required**)
- Azure TTS credentials (optional - Edge TTS works without configuration)

## Installation

1. Download the latest release
2. Extract to a folder of your choice
3. Run the installation helper in PowerShell:
   ```powershell
   .\install.ps1
   ```
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

## License

MIT License - see [LICENSE](LICENSE) file for details.
