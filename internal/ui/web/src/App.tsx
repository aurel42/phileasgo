import { useNavigate, useLocation } from 'react-router-dom';
import { InfoPanel } from './components/InfoPanel';
import { POIInfoPanel } from './components/POIInfoPanel';
import { PlaybackControls } from './components/PlaybackControls';
import { RegionalCategoriesCard } from './components/RegionalCategoriesCard';
import { SpatialFeaturesCard } from './components/SpatialFeaturesCard';
import { DashboardTabs, type TabId } from './components/DashboardTabs';
import { useTelemetry } from './hooks/useTelemetry';
import { useTrackedPOIs } from './hooks/usePOIs';
import type { POI } from './hooks/usePOIs';
import { useNarrator } from './hooks/useNarrator';
import { useBackendStats, useBackendVersion } from './hooks/useBackendInfo';
import { DashboardFooter } from './components/DashboardFooter';
import { POIsCard } from './components/POIsCard';
import { CitiesCard } from './components/CitiesCard';
import { useState, useEffect, useCallback, useRef, lazy, Suspense } from 'react';
import { type AircraftType } from './components/AircraftIcon';
import { useManualNarration } from './hooks/useManualNarration';

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
  const [repeatTTL, setRepeatTTL] = useState(3600);
  const [narrationLengthShort, setNarrationLengthShort] = useState(50);
  const [narrationLengthLong, setNarrationLengthLong] = useState(200);
  const [beaconMaxTargets, setBeaconMaxTargets] = useState(2);

  // Paper Opacity & Saturation (now persisted to backend)
  const [paperOpacityFog, setPaperOpacityFog] = useState(0.7);
  const [paperOpacityClear, setPaperOpacityClear] = useState(0.1);
  const [parchmentSaturation, setParchmentSaturation] = useState(1.0);

  // Debug: Artistic Map bounding boxes (localStorage-only)
  const [showArtisticDebugBoxes, setShowArtisticDebugBoxes] = useState(() => {
    return localStorage.getItem('showArtisticDebugBoxes') === 'true';
  });

  // Aircraft Customization (now persisted to backend)
  const [aircraftIcon, setAircraftIcon] = useState<AircraftType>('balloon');
  const [aircraftSize, setAircraftSize] = useState(32);
  const [aircraftColorMain, setAircraftColorMain] = useState('#fff');
  const [aircraftColorAccent, setAircraftColorAccent] = useState('#fff');

  // Volume
  const [volume, setVolume] = useState(1.0);

  const pois = useTrackedPOIs();
  const { status: narratorStatus } = useNarrator();
  const { data: backendStats } = useBackendStats();
  const { data: backendVersion } = useBackendVersion();
  const { playPOI, playCity, playFeature, statusMessage } = useManualNarration();

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
  const [activeTab, setActiveTab] = useState<TabId>('dashboard');
  const [previousTab, setPreviousTab] = useState<TabId>('dashboard');
  const autoOpenedRef = useRef(false);
  const lastAutoOpenedIdRef = useRef<string | null>(null); // Track last auto-opened narration to prevent re-switch loops

  // POIs are already filtered by the backend
  const bluePOIs = pois.filter(p => p.last_played && p.last_played !== "0001-01-01T00:00:00Z");
  const nonBlueCount = pois.length - bluePOIs.length;
  const blueCount = bluePOIs.length;

  // Auto-switch to POI tab when narrator has visual content worth showing.
  // Only switches for: POI narrations (with current_poi) and screenshots (with thumbnail).
  // Skips announcements (letsgo, border, debriefing, essay) that have no rich visual content.
  useEffect(() => {
    const playbackStatus = narratorStatus?.playback_status;

    // Phase 1: Preparing a POI — switch early if user is on a passive tab
    if (playbackStatus === 'preparing' && narratorStatus?.preparing_poi) {
      const poiId = narratorStatus.preparing_poi.wikidata_id;
      if (lastAutoOpenedIdRef.current === poiId) return;
      if (activeTab === 'diagnostics' || activeTab === 'detail') return;

      const poi = pois.find(p => p.wikidata_id === poiId);
      if (poi) {
        setPreviousTab(activeTab);
        setActiveTab('detail');
        setSelectedPOI(poi);
        autoOpenedRef.current = true;
        lastAutoOpenedIdRef.current = poiId;
      }
      return;
    }

    // Phase 2: Playing — only auto-switch for content-rich narration types
    if (playbackStatus !== 'playing' || !narratorStatus?.show_info_panel) return;
    if (activeTab === 'diagnostics') return;

    if (narratorStatus.current_type === 'poi' && narratorStatus.current_poi) {
      const poiId = narratorStatus.current_poi.wikidata_id;
      if (lastAutoOpenedIdRef.current === poiId) return;

      const poi = pois.find(p => p.wikidata_id === poiId);
      if (poi) {
        if (activeTab !== 'detail') {
          setPreviousTab(activeTab);
          setActiveTab('detail');
          autoOpenedRef.current = true;
        }
        setSelectedPOI(poi);
        lastAutoOpenedIdRef.current = poiId;
      }
      return;
    }

    if (narratorStatus.current_type === 'screenshot' && narratorStatus.display_thumbnail) {
      const key = 'screenshot-' + narratorStatus.current_title;
      if (lastAutoOpenedIdRef.current === key) return;

      if (activeTab !== 'detail') {
        setPreviousTab(activeTab);
        setActiveTab('detail');
        autoOpenedRef.current = true;
      }
      setSelectedPOI(null); // Generic panel renders the screenshot
      lastAutoOpenedIdRef.current = key;
      return;
    }

    // Other types (essay, letsgo, border, debriefing, briefing): no auto-switch
  }, [narratorStatus?.playback_status, narratorStatus?.preparing_poi,
      narratorStatus?.current_poi, narratorStatus?.current_type,
      narratorStatus?.current_title, narratorStatus?.show_info_panel,
      narratorStatus?.display_thumbnail, activeTab, pois]);

  // Revert tab when narration ends
  useEffect(() => {
    if (narratorStatus?.playback_status === 'idle' && autoOpenedRef.current && activeTab === 'detail' && previousTab !== 'detail') {
      setActiveTab(previousTab);
      autoOpenedRef.current = false;
      lastAutoOpenedIdRef.current = null; // Allow same POI to auto-open again next time
    }
  }, [narratorStatus?.playback_status, activeTab, previousTab]);

  // Handler for manual POI selection (from map marker click)
  const handlePOISelect = useCallback((poi: POI) => {
    if (activeTab === 'diagnostics') return;

    setSelectedPOI(poi);
    if (activeTab !== 'detail') {
      setPreviousTab(activeTab);
    }
    setActiveTab('detail');
    autoOpenedRef.current = false; // User manually selected — don't auto-revert
  }, [activeTab]);

  // Handler for closing the panel (e.g. from map click)
  const handlePanelClose = useCallback(() => {
    setSelectedPOI(null);
    autoOpenedRef.current = false;
    if (activeTab === 'detail') {
      setActiveTab(previousTab);
    }
  }, [activeTab, previousTab]);

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
    if (key === 'repeat_ttl') setRepeatTTL(value as number);
    if (key === 'narration_length_short_words') setNarrationLengthShort(value as number);
    if (key === 'narration_length_long_words') setNarrationLengthLong(value as number);
    if (key === 'settlement_label_limit') setSettlementLabelLimit(value as number);
    if (key === 'settlement_tier') setSettlementTier(value as number);
    if (key === 'paper_opacity_clear') setPaperOpacityClear(value as number);
    if (key === 'paper_opacity_fog') setPaperOpacityFog(value as number);
    if (key === 'parchment_saturation') setParchmentSaturation(value as number);
    if (key === 'aircraft_icon') setAircraftIcon(value as AircraftType);
    if (key === 'aircraft_size') setAircraftSize(value as number);
    if (key === 'aircraft_color_main') setAircraftColorMain(value as string);
    if (key === 'aircraft_color_accent') setAircraftColorAccent(value as string);
    if (key === 'volume') setVolume(value as number);

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
          setActiveMapStyle(data.active_map_style || 'dark');
          setMinPoiScore(data.min_poi_score ?? 0.5);
          setFilterMode(data.filter_mode || 'fixed');
          setTargetCount(data.target_poi_count ?? 20);

          // These two can also be driven by narratorStatus, but config is the source of truth for settings
          setNarrationFrequency(data.narration_frequency ?? 3);
          setTextLength(data.text_length ?? 3);
          setAutoNarrate(data.auto_narrate ?? true);
          setPauseDuration(data.pause_between_narrations ?? 4);
          setRepeatTTL(data.repeat_ttl ?? 3600);
          setNarrationLengthShort(data.narration_length_short_words ?? 50);
          setNarrationLengthLong(data.narration_length_long_words ?? 200);
          setSettlementLabelLimit(data.settlement_label_limit ?? 5);
          setSettlementTier(data.settlement_tier ?? 3);
          if (data.settlement_categories) setSettlementCategories(data.settlement_categories);
          if (data.beacon_max_targets !== undefined) setBeaconMaxTargets(data.beacon_max_targets);
          if (data.paper_opacity_clear !== undefined) setPaperOpacityClear(data.paper_opacity_clear);
          if (data.paper_opacity_fog !== undefined) setPaperOpacityFog(data.paper_opacity_fog);
          if (data.parchment_saturation !== undefined) setParchmentSaturation(data.parchment_saturation);
          if (data.aircraft_icon) setAircraftIcon(data.aircraft_icon);
          if (data.aircraft_size) setAircraftSize(data.aircraft_size);
          if (data.aircraft_color_main) setAircraftColorMain(data.aircraft_color_main);
          if (data.aircraft_color_accent) setAircraftColorAccent(data.aircraft_color_accent);
          if (data.volume !== undefined) setVolume(data.volume);
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
          onPaperOpacityFogChange={(val) => updateConfig('paper_opacity_fog', val)}
          paperOpacityClear={paperOpacityClear}
          onPaperOpacityClearChange={(val) => updateConfig('paper_opacity_clear', val)}
          parchmentSaturation={parchmentSaturation}
          onParchmentSaturationChange={(val) => updateConfig('parchment_saturation', val)}
          showArtisticDebugBoxes={showArtisticDebugBoxes}
          onShowArtisticDebugBoxesChange={(val) => {
            setShowArtisticDebugBoxes(val);
            localStorage.setItem('showArtisticDebugBoxes', String(val));
          }}
          volume={volume}
          onVolumeChange={(val) => updateConfig('volume', val)}
          aircraftIcon={aircraftIcon}
          onAircraftIconChange={(val) => updateConfig('aircraft_icon', val)}
          aircraftSize={aircraftSize}
          onAircraftSizeChange={(val) => updateConfig('aircraft_size', val)}
          aircraftColorMain={aircraftColorMain}
          onAircraftColorMainChange={(val) => updateConfig('aircraft_color_main', val)}
          aircraftColorAccent={aircraftColorAccent}
          onAircraftColorAccentChange={(val) => updateConfig('aircraft_color_accent', val)}
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
              beaconMaxTargets={beaconMaxTargets}
              isAutoOpened={autoOpenedRef.current}
              onPOISelect={handlePOISelect}
              onMapClick={handlePanelClose}
              showDebugBoxes={showArtisticDebugBoxes}
              aircraftIcon={aircraftIcon}
              aircraftSize={aircraftSize}
              aircraftColorMain={aircraftColorMain}
              aircraftColorAccent={aircraftColorAccent}
            />
          ) : (
            <Map
              units={units}
              showCacheLayer={showCacheLayer}
              showVisibilityLayer={showVisibilityLayer}
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
        <DashboardTabs activeTab={activeTab} onTabChange={setActiveTab} />

        {activeTab === 'dashboard' && (
          <InfoPanel
            activeTab="dashboard"
            telemetry={telemetry}
            status={hasConnectionError ? 'error' : status}
            isRetrying={status === 'pending' && hasConnectionError}
            stats={backendStats}
          />
        )}

        {activeTab === 'detail' && (
          <POIInfoPanel
            key={selectedPOI?.wikidata_id || (narratorStatus?.current_type + '-' + narratorStatus?.current_title)}
            poi={selectedPOI}
            pois={pois}
            currentTitle={narratorStatus?.current_title}
            currentType={narratorStatus?.current_type}
          />
        )}

        {activeTab === 'pois' && (
          <POIsCard telemetry={telemetry} onPlayPOI={playPOI} />
        )}

        {activeTab === 'cities' && (
          <CitiesCard telemetry={telemetry} onPlayCity={playCity} />
        )}

        {activeTab === 'regional' && (
          <>
            <RegionalCategoriesCard onPlayFeature={playFeature} />
            <SpatialFeaturesCard onPlayFeature={playFeature} />
          </>
        )}

        {activeTab === 'diagnostics' && (
          <InfoPanel
            activeTab="diagnostics"
            telemetry={telemetry}
            status={hasConnectionError ? 'error' : status}
            isRetrying={status === 'pending' && hasConnectionError}
            stats={backendStats}
          />
        )}

        <DashboardFooter
          telemetry={telemetry}
          stats={backendStats}
          version={backendVersion}
          nonBlueCount={nonBlueCount}
          blueCount={blueCount}
          minPoiScore={minPoiScore}
          targetCount={targetCount}
          filterMode={filterMode}
          narrationFrequency={narrationFrequency}
          textLength={textLength}
          onSettingsClick={() => navigate('/settings')}
        />
        {statusMessage && (
          <div style={{
            position: 'fixed',
            bottom: '80px',
            left: '50%',
            transform: 'translateX(-50%)',
            background: 'var(--accent)',
            color: 'white',
            padding: '8px 16px',
            borderRadius: '4px',
            boxShadow: '0 4px 12px rgba(0,0,0,0.5)',
            zIndex: 1000,
            fontSize: '14px',
            fontWeight: '600',
            animation: 'fadeInOut 3s ease-in-out'
          }}>
            {statusMessage}
          </div>
        )}
      </div>
    </div >
  );
}

export default App;
