import { Map } from './components/Map';
import { InfoPanel } from './components/InfoPanel';
import { POIInfoPanel } from './components/POIInfoPanel';
import { PlaybackControls } from './components/PlaybackControls';
import { useTelemetry } from './hooks/useTelemetry';
import { useTrackedPOIs } from './hooks/usePOIs';
import type { POI } from './hooks/usePOIs';
import { useNarrator } from './hooks/useNarrator';
import { useState, useEffect, useCallback, useRef } from 'react';

type Units = 'km' | 'nm';



function App() {
  // Streaming mode state (persisted to localStorage)
  const [streamingMode, setStreamingMode] = useState(() => {
    const saved = localStorage.getItem('streamingMode');
    return saved === 'true';
  });
  const { data: telemetry, status } = useTelemetry(streamingMode);
  const [units, setUnits] = useState<Units>('km');
  const [showCacheLayer, setShowCacheLayer] = useState(false);
  const [showVisibilityLayer, setShowVisibilityLayer] = useState(false);
  const [minPoiScore, setMinPoiScore] = useState(0.5);
  const [filterMode, setFilterMode] = useState<string>('fixed');
  const [targetCount, setTargetCount] = useState(20);
  const [narrationFrequency, setNarrationFrequency] = useState(3);
  const [textLength, setTextLength] = useState(3);
  const [isConfigOpen, setIsConfigOpen] = useState(false);
  const pois = useTrackedPOIs();
  const { status: narratorStatus } = useNarrator();

  // Connection error latching
  const [hasConnectionError, setHasConnectionError] = useState(false);

  useEffect(() => {
    if (status === 'error') {
      setHasConnectionError(true);
    } else if (status === 'success') {
      setHasConnectionError(false);
    }
  }, [status]);

  // POI selection state (lifted from Map.tsx)
  const [selectedPOI, setSelectedPOI] = useState<POI | null>(null);
  const autoOpenedRef = useRef(false);
  const userDismissedRef = useRef<string | null>(null); // Track ID of user-dismissed POI
  const lastAutoOpenedIdRef = useRef<string | null>(null); // Track ID of last auto-opened POI to prevent loops

  // POIs are already filtered by the backend
  const displayedPOIs = pois;
  const displayedCount = displayedPOIs.length;

  // Auto-open panel when narrator starts playing (unless user dismissed it)
  useEffect(() => {
    if (narratorStatus?.playback_status === 'playing' && narratorStatus?.current_poi) {
      const poiId = narratorStatus.current_poi.wikidata_id;
      // Don't auto-open if user manually closed the panel for THIS specific POI
      if (userDismissedRef.current === poiId) {
        return;
      }

      // Check if we already auto-opened this specific POI
      if (lastAutoOpenedIdRef.current === poiId) {
        return;
      }

      // DO NOT auto-open if the configuration panel is open
      if (isConfigOpen) {
        return;
      }

      // DO NOT auto-open if the user has manually selected a POI
      if (selectedPOI && !autoOpenedRef.current) {
        return;
      }

      const poi = pois.find(p => p.wikidata_id === poiId);
      if (poi && selectedPOI?.wikidata_id !== poiId) {
        setSelectedPOI(poi);
        autoOpenedRef.current = true;
        lastAutoOpenedIdRef.current = poiId;
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [narratorStatus?.playback_status, narratorStatus?.current_poi?.wikidata_id, pois]);

  // Auto-close panel when narrator stops (only if auto-opened)
  useEffect(() => {
    if (narratorStatus?.playback_status === 'idle' && autoOpenedRef.current) {
      setSelectedPOI(null);
      autoOpenedRef.current = false;
      lastAutoOpenedIdRef.current = null;
    }
  }, [narratorStatus?.playback_status]);

  // Handler for manual POI selection (from map)
  const handlePOISelect = useCallback((poi: POI) => {
    setSelectedPOI(poi);
    autoOpenedRef.current = false; // User manually selected, don't auto-close
    userDismissedRef.current = null; // New selection, reset dismissed suppression
  }, []);

  // Handler for closing the panel
  const handlePanelClose = useCallback(() => {
    if (selectedPOI) {
      userDismissedRef.current = selectedPOI.wikidata_id; // Suppress auto-open for this POI
    }
    setSelectedPOI(null);
    autoOpenedRef.current = false;
  }, [selectedPOI]);

  // Helper to update config via API
  const updateConfig = useCallback((key: string, value: string | number | boolean) => {
    // Optimistic update
    if (key === 'units') setUnits(value as Units);
    if (key === 'show_cache_layer') setShowCacheLayer(value as boolean);
    if (key === 'show_visibility_layer') setShowVisibilityLayer(value as boolean);
    if (key === 'min_poi_score') setMinPoiScore(value as number);
    if (key === 'filter_mode') setFilterMode(value as string);
    if (key === 'target_poi_count') setTargetCount(value as number);
    if (key === 'narration_frequency') setNarrationFrequency(value as number);
    if (key === 'text_length') setTextLength(value as number);

    fetch('/api/config', {
      method: 'PUT', // Changed from POST to PUT for consistency with existing handlers
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ [key]: value })
    }).catch(e => {
      console.error("Failed to save config", e);
      // Revert on error would be ideal here
    });
  }, []);

  // Fetch config on mount
  useEffect(() => {
    fetch('/api/config')
      .then(r => r.json())
      .then(data => {
        setUnits(data.units || 'km');
        setShowCacheLayer(data.show_cache_layer || false);
        setShowVisibilityLayer(data.show_visibility_layer || false);
        setMinPoiScore(data.min_poi_score ?? 0.5);
        setFilterMode(data.filter_mode || 'fixed');
        setTargetCount(data.target_poi_count ?? 20);
        setNarrationFrequency(data.narration_frequency ?? 3);
        setTextLength(data.text_length ?? 3);
      })
      .catch(e => console.error("Failed to fetch config", e));
  }, []);



  // Handler to update visibility layer config
  const handleVisibilityLayerChange = useCallback((show: boolean) => {
    setShowVisibilityLayer(show);
    fetch('/api/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ show_visibility_layer: show })
    }).catch(e => console.error("Failed to update visibility layer config", e));
  }, []);

  // Handler to update min poi score config
  const handleMinPoiScoreChange = useCallback((score: number) => {
    setMinPoiScore(score);
    fetch('/api/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ min_poi_score: score })
    }).catch(e => console.error("Failed to update min poi score", e));
  }, []);

  const handleFilterModeChange = useCallback((mode: string) => {
    setFilterMode(mode);
    fetch('/api/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ filter_mode: mode })
    }).catch(e => console.error("Failed to update filter mode", e));
  }, []);

  const handleTargetCountChange = useCallback((count: number) => {
    setTargetCount(count);
    fetch('/api/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ target_poi_count: count })
    }).catch(e => console.error("Failed to update target count", e));
  }, []);

  return (
    <div className="app-container">
      <div className="map-container">
        <Map
          units={units}
          showCacheLayer={showCacheLayer}
          showVisibilityLayer={showVisibilityLayer}
          pois={pois}
          selectedPOI={selectedPOI}
          onPOISelect={handlePOISelect}
          onMapClick={handlePanelClose}
        />
        {hasConnectionError && (
          <div className="connection-warning">
            ⚠️ Connection lost, trying to reconnect...
          </div>
        )}
      </div>
      <div className="dashboard-container">
        <PlaybackControls />
        {selectedPOI ? (
          <POIInfoPanel
            key={selectedPOI.wikidata_id}
            poi={selectedPOI}
            pois={pois}
            aircraftHeading={telemetry?.Heading || 0}
            onClose={handlePanelClose}
          />
        ) : (
          <InfoPanel
            telemetry={telemetry}
            status={hasConnectionError ? 'error' : status}
            isRetrying={status === 'pending' && hasConnectionError}
            units={units}
            onUnitsChange={(val) => updateConfig('units', val)}
            showCacheLayer={showCacheLayer}
            onCacheLayerChange={(val) => updateConfig('show_cache_layer', val)}
            showVisibilityLayer={showVisibilityLayer}
            onVisibilityLayerChange={handleVisibilityLayerChange}
            displayedCount={displayedCount}
            minPoiScore={minPoiScore}
            onMinPoiScoreChange={handleMinPoiScoreChange}
            filterMode={filterMode}
            onFilterModeChange={handleFilterModeChange}
            targetPoiCount={targetCount}
            onTargetPoiCountChange={handleTargetCountChange}
            narrationFrequency={narrationFrequency}
            onNarrationFrequencyChange={(val) => updateConfig('narration_frequency', val)}
            textLength={textLength}
            onTextLengthChange={(val) => updateConfig('text_length', val)}
            streamingMode={streamingMode}
            onStreamingModeChange={(val) => {
              setStreamingMode(val);
              localStorage.setItem('streamingMode', String(val));
            }}
            isConfigOpen={isConfigOpen}
            onConfigOpenChange={setIsConfigOpen}
          />
        )}
      </div>

    </div>
  );
}

export default App;
