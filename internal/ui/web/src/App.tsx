import { useNavigate, useLocation } from 'react-router-dom';
import { InfoPanel } from './components/InfoPanel';
import { POIInfoPanel } from './components/POIInfoPanel';
import { PlaybackControls } from './components/PlaybackControls';
import { useTelemetry } from './hooks/useTelemetry';
import { useTrackedPOIs } from './hooks/usePOIs';
import type { POI } from './hooks/usePOIs';
import { useNarrator } from './hooks/useNarrator';
import { useState, useEffect, useCallback, useRef, lazy, Suspense } from 'react';

// Lazy load heavy map components
const Map = lazy(() => import('./components/Map').then(m => ({ default: m.Map })));
const ArtisticMap = lazy(() => import('./components/ArtisticMap').then(m => ({ default: m.ArtisticMap })));
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
  const [renderVisibilityAsMap, setRenderVisibilityAsMap] = useState(false);
  const [activeMapStyle, setActiveMapStyle] = useState('dark');
  const [minPoiScore, setMinPoiScore] = useState(0.5);
  const [filterMode, setFilterMode] = useState<string>('fixed');
  const [targetCount, setTargetCount] = useState(20);
  const [settlementLabelLimit, setSettlementLabelLimit] = useState(5);
  const [settlementTier, setSettlementTier] = useState(3);
  const [settlementCategories, setSettlementCategories] = useState<string[]>([]);
  const [narrationFrequency, setNarrationFrequency] = useState(3);
  const [textLength, setTextLength] = useState(3);
  const [autoNarrate, setAutoNarrate] = useState(true);
  const [pauseDuration, setPauseDuration] = useState(4);
  const [repeatTTL, setRepeatTTL] = useState('1h');
  const [narrationLengthShort, setNarrationLengthShort] = useState(50);
  const [narrationLengthLong, setNarrationLengthLong] = useState(200);

  // Paper Opacity State (init with defaults 0.7 and 0.1)
  const [paperOpacityFog, setPaperOpacityFog] = useState(() => {
    const saved = localStorage.getItem('paperOpacityFog');
    return saved ? parseFloat(saved) : 0.7;
  });
  const [paperOpacityClear, setPaperOpacityClear] = useState(() => {
    const saved = localStorage.getItem('paperOpacityClear');
    return saved ? parseFloat(saved) : 0.1;
  });

  // Parchment Saturation State (init with default 1.0)
  const [parchmentSaturation, setParchmentSaturation] = useState(() => {
    const saved = localStorage.getItem('parchmentSaturation');
    return saved ? parseFloat(saved) : 1.0;
  });

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
    if (narratorStatus?.playback_status === 'playing' && narratorStatus?.show_info_panel) {
      if (narratorStatus.current_type === 'poi' && narratorStatus.current_poi) {
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
      } else {
        // Auto-open for non-POI narratives (managed by show_info_panel flag from backend)
        const title = narratorStatus.display_title || narratorStatus.current_title || narratorStatus.current_type || 'Narration';
        if (lastAutoOpenedIdRef.current !== title) {
          lastAutoOpenedIdRef.current = title;
          // Clear any previous POI selection to ensure the generic panel shows instead
          setSelectedPOI(null);
          setShowGenericPanel(true);
          autoOpenedRef.current = true;
        }
      }
    }
  }, [narratorStatus?.playback_status, narratorStatus?.current_poi?.wikidata_id, narratorStatus?.current_type, narratorStatus?.current_title, narratorStatus?.show_info_panel, pois]);

  // Auto-close panel when narrator stops or switches to content that shouldn't show the panel
  useEffect(() => {
    const isIdle = narratorStatus?.playback_status === 'idle';
    const shouldShow = narratorStatus?.show_info_panel ?? false;

    if ((isIdle || !shouldShow) && autoOpenedRef.current) {
      setSelectedPOI(null);
      setShowGenericPanel(false);
      autoOpenedRef.current = false;
      lastAutoOpenedIdRef.current = null;
    }
  }, [narratorStatus?.playback_status, narratorStatus?.show_info_panel]);

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
    if (key === 'auto_narrate') setAutoNarrate(value as boolean);
    if (key === 'pause_between_narrations') setPauseDuration(value as number);
    if (key === 'repeat_ttl') setRepeatTTL(value as string);
    if (key === 'narration_length_short_words') setNarrationLengthShort(value as number);
    if (key === 'narration_length_long_words') setNarrationLengthLong(value as number);
    if (key === 'settlement_label_limit') setSettlementLabelLimit(value as number);
    if (key === 'settlement_tier') setSettlementTier(value as number);

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
          setUnits(data.range_ring_units || 'km');
          setShowCacheLayer(data.show_cache_layer || false);
          setShowVisibilityLayer(data.show_visibility_layer || false);
          setRenderVisibilityAsMap(data.render_visibility_as_map || false);
          setActiveMapStyle(data.active_map_style || 'dark');
          setMinPoiScore(data.min_poi_score ?? 0.5);
          setFilterMode(data.filter_mode || 'fixed');
          setTargetCount(data.target_poi_count ?? 20);

          // These two can also be driven by narratorStatus, but config is the source of truth for settings
          setNarrationFrequency(data.narration_frequency ?? 3);
          setTextLength(data.text_length ?? 3);
          setAutoNarrate(data.auto_narrate ?? true);
          setPauseDuration(data.pause_between_narrations ?? 4);
          setRepeatTTL(data.repeat_ttl || '1h');
          setNarrationLengthShort(data.narration_length_short_words ?? 50);
          setNarrationLengthLong(data.narration_length_long_words ?? 200);
          setSettlementLabelLimit(data.settlement_label_limit ?? 5);
          setSettlementTier(data.settlement_tier ?? 3);
          if (data.settlement_categories) setSettlementCategories(data.settlement_categories);
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

  const handleRenderVisibilityAsMapChange = useCallback((render: boolean) => {
    setRenderVisibilityAsMap(render);
    fetch('/api/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ render_visibility_as_map: render })
    }).catch(e => console.error("Failed to update visibility rendering config", e));
  }, []);

  const handleActiveMapStyleChange = useCallback((style: string) => {
    setActiveMapStyle(style);
    fetch('/api/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ active_map_style: style })
    }).catch(e => console.error("Failed to update map style", e));
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
          onUnitsChange={(val) => updateConfig('range_ring_units', val)}
          showCacheLayer={showCacheLayer}
          onCacheLayerChange={(val) => updateConfig('show_cache_layer', val)}
          showVisibilityLayer={showVisibilityLayer}
          onVisibilityLayerChange={handleVisibilityLayerChange}
          renderVisibilityAsMap={renderVisibilityAsMap}
          onRenderVisibilityAsMapChange={handleRenderVisibilityAsMapChange}
          activeMapStyle={activeMapStyle}
          onActiveMapStyleChange={handleActiveMapStyleChange}
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
          autoNarrate={autoNarrate}
          onAutoNarrateChange={(val) => updateConfig('auto_narrate', val)}
          pauseDuration={pauseDuration}
          onPauseDurationChange={(val) => updateConfig('pause_between_narrations', val)}
          repeatTTL={repeatTTL}
          onRepeatTTLChange={(val) => updateConfig('repeat_ttl', val)}
          narrationLengthShort={narrationLengthShort}
          narrationLengthLong={narrationLengthLong}
          onNarrationLengthChange={(min, max) => {
            updateConfig('narration_length_short_words', min);
            updateConfig('narration_length_long_words', max);
          }}
          streamingMode={streamingMode}
          onStreamingModeChange={(val) => {
            setStreamingMode(val);
            localStorage.setItem('streamingMode', String(val));
          }}
          settlementLabelLimit={settlementLabelLimit}
          onSettlementLabelLimitChange={(val) => updateConfig('settlement_label_limit', val)}
          settlementTier={settlementTier}
          onSettlementTierChange={(val) => updateConfig('settlement_tier', val)}
          paperOpacityFog={paperOpacityFog}
          onPaperOpacityFogChange={(val) => {
            setPaperOpacityFog(val);
            localStorage.setItem('paperOpacityFog', String(val));
          }}
          paperOpacityClear={paperOpacityClear}
          onPaperOpacityClearChange={(val) => {
            setPaperOpacityClear(val);
            localStorage.setItem('paperOpacityClear', String(val));
          }}
          parchmentSaturation={parchmentSaturation}
          onParchmentSaturationChange={(val) => {
            setParchmentSaturation(val);
            localStorage.setItem('parchmentSaturation', String(val));
          }}
        />
      </Suspense>
    );
  }
  return (
    <div className="app-container">
      <div className="map-container">
        <Suspense fallback={<div style={{ width: '100%', height: '100%', background: '#000' }} />}>
          {activeMapStyle === 'artistic' ? (
            <ArtisticMap
              center={telemetry ? [telemetry.Latitude, telemetry.Longitude] : [0, 0]}
              zoom={10}
              className="w-full h-full"
              telemetry={telemetry ?? null}
              pois={pois}
              settlementTier={settlementTier}
              settlementCategories={settlementCategories}
              paperOpacityFog={paperOpacityFog}
              paperOpacityClear={paperOpacityClear}
              parchmentSaturation={parchmentSaturation}
              selectedPOI={selectedPOI}
              isAutoOpened={autoOpenedRef.current}
              onPOISelect={handlePOISelect}
              onMapClick={handlePanelClose}
            />
          ) : (
            <Map
              units={units}
              showCacheLayer={showCacheLayer}
              showVisibilityLayer={showVisibilityLayer}
              renderVisibilityAsMap={renderVisibilityAsMap}
              pois={pois}
              selectedPOI={selectedPOI}
              onPOISelect={handlePOISelect}
              onMapClick={handlePanelClose}
            />
          )}
        </Suspense>

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
            telemetry={telemetry ?? undefined}
            aircraftHeading={telemetry?.Heading || 0}
            currentTitle={narratorStatus?.current_title}
            currentType={narratorStatus?.current_type}
            onClose={handlePanelClose}
            minPoiScore={minPoiScore}
            targetCount={targetCount}
            filterMode={filterMode}
            narrationFrequency={narrationFrequency}
            textLength={textLength}
            onSettingsClick={() => navigate('/settings')}
          />
        ) : (
          <InfoPanel
            telemetry={telemetry}
            status={hasConnectionError ? 'error' : status}
            isRetrying={status === 'pending' && hasConnectionError}
            nonBlueCount={nonBlueCount}
            blueCount={blueCount}
            minPoiScore={minPoiScore}
            targetCount={targetCount}
            filterMode={filterMode}
            narrationFrequency={narrationFrequency}
            textLength={textLength}
            onSettingsClick={() => navigate('/settings')}
          />
        )}
      </div>

    </div >
  );
}

export default App;
