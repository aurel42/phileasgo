import { Map } from './components/Map';
import { useNavigate, useLocation } from 'react-router-dom';
import { InfoPanel } from './components/InfoPanel';
import { POIInfoPanel } from './components/POIInfoPanel';
import { PlaybackControls } from './components/PlaybackControls';
import { useTelemetry } from './hooks/useTelemetry';
import { useTrackedPOIs } from './hooks/usePOIs';
import type { POI } from './hooks/usePOIs';
import { useNarrator } from './hooks/useNarrator';
import { useState, useEffect, useCallback, useRef, lazy, Suspense } from 'react';

const SettingsPanel = lazy(() => import('./components/SettingsPanel').then(m => ({ default: m.SettingsPanel })));




type Units = 'km' | 'nm';



function App() {
  const navigate = useNavigate();
  const location = useLocation();

  const isGui = new URLSearchParams(window.location.search).get('gui') === 'true';


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
  const [showGenericPanel, setShowGenericPanel] = useState(false);
  const autoOpenedRef = useRef(false);
  const userDismissedRef = useRef<string | null>(null); // Track ID of user-dismissed POI
  const lastAutoOpenedIdRef = useRef<string | null>(null); // Track ID of last auto-opened POI to prevent loops

  // POIs are already filtered by the backend
  const bluePOIs = pois.filter(p => p.last_played && p.last_played !== "0001-01-01T00:00:00Z");
  const nonBlueCount = pois.length - bluePOIs.length;
  const blueCount = bluePOIs.length;

  // Auto-open panel when narrator starts playing (unless user dismissed it)
  useEffect(() => {
    if (narratorStatus?.playback_status === 'playing' && narratorStatus?.current_poi && narratorStatus?.current_type === 'poi') {
      const poiId = narratorStatus.current_poi.wikidata_id;
      // Don't auto-open if user manually closed the panel for THIS specific POI
      if (userDismissedRef.current === poiId) {
        return;
      }

      // Check if we already auto-opened this specific POI
      if (lastAutoOpenedIdRef.current === poiId) {
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
    } else if (narratorStatus?.playback_status === 'playing' && (narratorStatus?.current_type === 'debriefing' || narratorStatus?.current_type === 'essay' || narratorStatus?.current_type === 'screenshot')) {
      // Auto-open for non-POI narratives
      const title = narratorStatus.current_title ||
        (narratorStatus.current_type === 'debriefing' ? 'Debrief' :
          narratorStatus.current_type === 'screenshot' ? 'Photograph Analysis' : 'Essay');
      if (lastAutoOpenedIdRef.current !== title) {
        lastAutoOpenedIdRef.current = title;
        // Clear any previous POI selection to ensure the generic panel shows instead
        setSelectedPOI(null);
        setShowGenericPanel(true);
        autoOpenedRef.current = true;
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [narratorStatus?.playback_status, narratorStatus?.current_poi?.wikidata_id, narratorStatus?.current_type, narratorStatus?.current_title, pois]);

  // Auto-close panel when narrator stops or switches to non-POI content (e.g. screenshot)
  useEffect(() => {
    const isIdle = narratorStatus?.playback_status === 'idle';
    const isPlayingNonPoi = narratorStatus?.playback_status === 'playing' && !narratorStatus?.current_poi;
    const isSpecialType = narratorStatus?.current_type === 'debriefing' || narratorStatus?.current_type === 'essay' || narratorStatus?.current_type === 'screenshot';

    if ((isIdle || (isPlayingNonPoi && !isSpecialType)) && autoOpenedRef.current) {
      setSelectedPOI(null);
      setShowGenericPanel(false);
      autoOpenedRef.current = false;
      lastAutoOpenedIdRef.current = null;
    }
  }, [narratorStatus?.playback_status, narratorStatus?.current_poi, narratorStatus?.current_type]);

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
    setShowGenericPanel(false);
    autoOpenedRef.current = false;
  }, [selectedPOI]);

  // Fetch config on mount and poll for updates (to handle multi-tab/GUI changes)
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
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ [key]: value })
    }).catch(e => {
      console.error("Failed to save config", e);
    });
  }, []);

  useEffect(() => {
    const fetchConfig = () => {
      fetch('/api/config')
        .then(r => r.json())
        .then(data => {
          // Only update if changed to avoid unnecessary re-renders? 
          // React state updates are cheap if value is same (bailout), but let's just set them.
          setUnits(data.units || 'km');
          setShowCacheLayer(data.show_cache_layer || false);
          setShowVisibilityLayer(data.show_visibility_layer || false);
          setMinPoiScore(data.min_poi_score ?? 0.5);
          setFilterMode(data.filter_mode || 'fixed');
          setTargetCount(data.target_poi_count ?? 20);

          // These two can also be driven by narratorStatus, but config is the source of truth for settings
          setNarrationFrequency(data.narration_frequency ?? 3);
          setTextLength(data.text_length ?? 3);
        })
        .catch(e => console.error("Failed to fetch config", e));
    };

    fetchConfig(); // Initial fetch
    const interval = setInterval(fetchConfig, 2000); // Poll every 2s
    return () => clearInterval(interval);
  }, []);

  // Sync config from narrator status (Transponder updates)
  useEffect(() => {
    if (narratorStatus?.narration_frequency !== undefined && navigator.onLine) {
      if (narratorStatus.narration_frequency !== narrationFrequency) {
        setNarrationFrequency(narratorStatus.narration_frequency);
      }
    }
    if (narratorStatus?.text_length !== undefined && navigator.onLine) {
      if (narratorStatus.text_length !== textLength) {
        setTextLength(narratorStatus.text_length);
      }
    }
  }, [narratorStatus?.narration_frequency, narratorStatus?.text_length, narrationFrequency, textLength]);

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

  // Simple Router Check
  const isSettings = location.pathname === '/settings';

  if (isSettings) {
    return (
      <Suspense fallback={<div style={{ background: '#060606', height: '100vh' }} />}>
        <SettingsPanel
          isGui={isGui}
          onBack={() => navigate('/')}
          telemetry={telemetry ?? null}
          units={units}
          onUnitsChange={(val) => updateConfig('units', val)}
          showCacheLayer={showCacheLayer}
          onCacheLayerChange={(val) => updateConfig('show_cache_layer', val)}
          showVisibilityLayer={showVisibilityLayer}
          onVisibilityLayerChange={handleVisibilityLayerChange}
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
        />
      </Suspense>
    );
  }
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

        {/* Config Pill Overlay */}
        <div className="config-pill" onClick={() => navigate('/settings')} style={{
          position: 'absolute',
          top: '20px',
          right: '20px',
          zIndex: 1000,
          textDecoration: 'none',
          color: 'inherit',
          background: 'var(--panel-bg)',
          boxShadow: '0 4px 10px rgba(0,0,0,0.5)',
          cursor: 'pointer'
        }}>
          {/* Sim Status Item */}
          <div className="config-pill-item" style={{ marginRight: '8px', paddingRight: '12px', borderRight: '1px solid rgba(255,255,255,0.1)' }}>
            <div className="status-dot" style={{
              width: '8px',
              height: '8px',
              marginRight: '6px',
              backgroundColor: !telemetry || telemetry.SimState === 'disconnected' ? '#ef4444' : (telemetry.SimState === 'inactive' ? '#fbbf24' : '#22c55e')
            }}></div>
            <span className="role-label" style={{ color: 'var(--text-color)', textTransform: 'uppercase' }}>
              {!telemetry ? 'DISCONNECTED' : telemetry.SimState}
            </span>
          </div>

          <div className="config-pill-item">
            <span className="config-mode-icon" style={{ color: 'var(--accent)' }}>{filterMode === 'adaptive' ? '⚡' : '🎯'}</span>
            <span className="role-label" style={{ color: 'var(--muted)' }}>
              {filterMode === 'adaptive' ? targetCount : minPoiScore}
            </span>
          </div>
          <div className="config-pill-item">
            <span className="role-label" style={{ color: 'var(--muted)', marginRight: '6px' }}>FRQ</span>
            <div className="pip-container">
              {[1, 2, 3, 4, 5].map(v => (
                <div key={v} className={`pip ${(narrationFrequency || 0) >= v ? 'active' : ''} ${(narrationFrequency || 0) >= v && v > 3 ? 'high' : ''}`} />
              ))}
            </div>
          </div>
          <div className="config-pill-item">
            <span className="role-label" style={{ color: 'var(--muted)', marginRight: '6px' }}>LEN</span>
            <div className="pip-container">
              {[1, 2, 3, 4, 5].map(v => (
                <div key={v} className={`pip ${(textLength || 0) >= v ? 'active' : ''} ${(textLength || 0) >= v && v > 4 ? 'high' : ''}`} />
              ))}
            </div>
          </div>
        </div>

        {hasConnectionError && (
          <div className="connection-warning">
            ⚠️ Connection lost, trying to reconnect...
          </div>
        )}
      </div>
      <div className="dashboard-container">
        <PlaybackControls />
        {(selectedPOI || showGenericPanel) ? (
          <POIInfoPanel
            key={selectedPOI?.wikidata_id || (narratorStatus?.current_type + '-' + narratorStatus?.current_title)}
            poi={selectedPOI}
            pois={pois}
            aircraftHeading={telemetry?.Heading || 0}
            currentTitle={narratorStatus?.current_title}
            currentType={narratorStatus?.current_type}
            onClose={handlePanelClose}
          />
        ) : (
          <InfoPanel
            telemetry={telemetry}
            status={hasConnectionError ? 'error' : status}
            isRetrying={status === 'pending' && hasConnectionError}
            nonBlueCount={nonBlueCount}
            blueCount={blueCount}
          />
        )}
      </div>

    </div >
  );
}

export default App;
