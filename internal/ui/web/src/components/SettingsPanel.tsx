import React, { useState, useEffect } from 'react';
import { VictorianToggle } from './VictorianToggle';
import { DualRangeSlider } from './DualRangeSlider';
import type { Telemetry } from '../types/telemetry';

import { AircraftIcon, type AircraftType } from './AircraftIcon';

const VICTORIAN_PALETTE = [
    // Row 1: Reds & Pinks
    '#590d22', '#800f2f', '#a4133c', '#c9184a', '#ff4d6d', '#ff758f', '#ff8fa3', '#ffb3c1',
    // Row 2: Oranges & Peaches
    '#e85d04', '#f48c06', '#faa307', '#ffba08', '#fcbf49', '#eae2b7', '#fcd5ce', '#f8edeb',
    // Row 3: Yellows & Golds
    '#ffcdb2', '#ffb4a2', '#e5989b', '#b5838d', '#6d597a', '#ffd60a', '#ffcc00', '#d4d700',
    // Row 4: Greens
    '#004b23', '#006400', '#007200', '#008000', '#38b000', '#70e000', '#9ef01a', '#ccff33',
    // Row 5: Teals & Cyans
    '#0c4a6e', '#075985', '#0ea5e9', '#38bdf8', '#7dd3fc', '#bae6fd', '#e0f2fe', '#f0f9ff',
    // Row 6: Blues
    '#03045e', '#023e8a', '#0077b6', '#0096c7', '#00b4d8', '#48cae4', '#90e0ef', '#caf0f8',
    // Row 7: Purples & Violets
    '#10002b', '#240046', '#3c096c', '#5a189a', '#7b2cbf', '#9d4edd', '#c77dff', '#e0aaff',
    // Row 8: Monochromes & Browns
    '#000000', '#1a1a1a', '#333333', '#4d4d4d', '#666666', '#808080', '#999999', '#cccccc',
];

/** Convert seconds to a compact human-readable string (e.g. 86400 → "1d", 3600 → "1h"). */
function secondsToHuman(secs: number): string {
    if (secs <= 0) return '0s';
    const d = Math.floor(secs / 86400);
    const h = Math.floor((secs % 86400) / 3600);
    const m = Math.floor((secs % 3600) / 60);
    const s = secs % 60;
    const parts: string[] = [];
    if (d) parts.push(`${d}d`);
    if (h) parts.push(`${h}h`);
    if (m) parts.push(`${m}m`);
    if (s || parts.length === 0) parts.push(`${s}s`);
    return parts.join('');
}

/** Parse a human-readable duration string (e.g. "1d", "2h30m", "90s") to seconds. Returns null on failure. */
function parseHumanToSeconds(input: string): number | null {
    const s = input.trim();
    if (!s) return null;
    // Try plain number (interpret as seconds)
    if (/^\d+$/.test(s)) return parseInt(s, 10);
    const re = /(\d+(?:\.\d+)?)\s*(d|h|m|s)/gi;
    let total = 0;
    let matched = false;
    let match: RegExpExecArray | null;
    while ((match = re.exec(s)) !== null) {
        matched = true;
        const val = parseFloat(match[1]);
        switch (match[2].toLowerCase()) {
            case 'd': total += val * 86400; break;
            case 'h': total += val * 3600; break;
            case 'm': total += val * 60; break;
            case 's': total += val; break;
        }
    }
    return matched ? Math.round(total) : null;
}

interface SettingsPanelProps {
    isGui: boolean;
    onBack: () => void;
    telemetry: Telemetry | null;
    units: 'km' | 'nm';
    onUnitsChange: (val: 'km' | 'nm') => void;
    showCacheLayer: boolean;
    onCacheLayerChange: (val: boolean) => void;
    showVisibilityLayer: boolean;
    onVisibilityLayerChange: (val: boolean) => void;
    activeMapStyle: string;
    onActiveMapStyleChange: (val: string) => void;
    minPoiScore: number;
    onMinPoiScoreChange: (val: number) => void;
    filterMode: string;
    onFilterModeChange: (val: string) => void;
    targetPoiCount: number;
    onTargetPoiCountChange: (val: number) => void;
    narrationFrequency: number;
    onNarrationFrequencyChange: (val: number) => void;
    textLength: number;
    onTextLengthChange: (val: number) => void;
    autoNarrate: boolean;
    onAutoNarrateChange: (val: boolean) => void;
    pauseDuration: number;
    onPauseDurationChange: (val: number) => void;
    repeatTTL: number;
    onRepeatTTLChange: (val: number) => void;
    narrationLengthShort: number;
    narrationLengthLong: number;
    onNarrationLengthChange: (minValue: number, maxValue: number) => void;
    streamingMode: boolean;
    onStreamingModeChange: (val: boolean) => void;
    settlementLabelLimit: number;
    onSettlementLabelLimitChange: (val: number) => void;
    settlementTier: number;
    onSettlementTierChange: (val: number) => void;
    paperOpacityFog: number;
    onPaperOpacityFogChange: (val: number) => void;
    paperOpacityClear: number;
    onPaperOpacityClearChange: (val: number) => void;
    parchmentSaturation: number;
    onParchmentSaturationChange: (val: number) => void;
    showArtisticDebugBoxes: boolean;
    onShowArtisticDebugBoxesChange: (val: boolean) => void;
    // Audio
    volume: number;
    onVolumeChange: (val: number) => void;
    // Aircraft
    aircraftIcon: string;
    onAircraftIconChange: (val: string) => void;
    aircraftSize: number;
    onAircraftSizeChange: (val: number) => void;
    aircraftColorMain: string;
    onAircraftColorMainChange: (val: string) => void;
    aircraftColorAccent: string;
    onAircraftColorAccentChange: (val: string) => void;
}

const VictorianListEditor: React.FC<{
    values: string[];
    onChange: (newValues: string[]) => void;
    placeholder?: string;
}> = ({ values = [], onChange, placeholder }) => {
    const [inputValue, setInputValue] = useState('');

    const addTag = (val: string) => {
        const trimmed = val.trim();
        if (trimmed && !values.includes(trimmed)) {
            onChange([...values, trimmed]);
        }
        setInputValue('');
    };

    const removeTag = (index: number) => {
        onChange(values.filter((_, i) => i !== index));
    };

    return (
        <div className="v-list-editor">
            <div className="v-tags">
                {values.map((v, i) => (
                    <div key={i} className="v-tag">
                        <span>{v}</span>
                        <button onClick={() => removeTag(i)}>&times;</button>
                    </div>
                ))}
            </div>
            <div className="v-input-row">
                <input
                    type="text"
                    className="settings-input"
                    placeholder={placeholder}
                    value={inputValue}
                    onChange={e => setInputValue(e.target.value)}
                    onKeyDown={e => e.key === 'Enter' && addTag(inputValue)}
                />
                <button className="settings-back-btn" style={{ marginTop: 0, padding: '4px 12px' }} onClick={() => addTag(inputValue)}>+</button>
            </div>
        </div>
    );
};

// Draft state for all settings that need save/discard
interface DraftState {
    // Narrator tab
    narrationFrequency: number;
    textLength: number;
    autoNarrate: boolean;
    pauseDuration: number;
    repeatTTL: number;
    repeatTTLInput: string; // human-readable display/edit string
    narrationLengthShort: number;
    narrationLengthLong: number;
    promptUnits: string; // imperial/hybrid/metric - affects prompt templates
    minPoiScore: number;
    filterMode: string;
    targetPoiCount: number;
    activeStyle: string;
    styleLibrary: string[];
    activeSecretWord: string;
    secretWordLibrary: string[];
    activeTargetLanguage: string;
    targetLanguageLibrary: string[];
    // Sim tab
    simSource: string;
    mockStartLat: number | null;
    mockStartLon: number | null;
    mockStartAlt: number | null;
    mockStartHeading: number | null;
    mockDurationParked: number;
    mockDurationParkedInput: string;
    mockDurationTaxi: number;
    mockDurationTaxiInput: string;
    mockDurationHold: number;
    mockDurationHoldInput: string;
    // Interface tab (local-only, no server sync needed)
    units: 'km' | 'nm';
    showCacheLayer: boolean;
    showVisibilityLayer: boolean;
    activeMapStyle: string;
    streamingMode: boolean;
    // Scorer tab
    deferralProximityBoostPower: number;
    deferralThreshold: number;
    // Narrator refinement
    twoPassScriptGeneration: boolean;
    // Beacon tab
    beaconEnabled: boolean;
    beaconFormationEnabled: boolean;
    beaconFormationDistance: number;
    beaconFormationCount: number;
    beaconMinSpawnAltitude: number;
    beaconAltitudeFloor: number;
    beaconSinkDistanceFar: number;
    beaconSinkDistanceClose: number;
    beaconTargetFloorAGL: number;

    beaconMaxTargets: number;
    settlementLabelLimit: number;
    settlementTier: number;
    paperOpacityFog: number;
    paperOpacityClear: number;
    parchmentSaturation: number;
    // Debugging tab
    showArtisticDebugBoxes: boolean;
    // Aircraft (Scorer Tab)
    aircraftIcon: AircraftType;
    aircraftSize: number;
    aircraftColorMain: string;
    aircraftColorAccent: string;
    // Audio
    volume: number;
}

export const SettingsPanel: React.FC<SettingsPanelProps> = ({
    onBack,
    units,
    onUnitsChange,
    showCacheLayer,
    onCacheLayerChange,
    showVisibilityLayer,
    onVisibilityLayerChange,
    minPoiScore,
    onMinPoiScoreChange,
    filterMode,
    onFilterModeChange,
    targetPoiCount,
    onTargetPoiCountChange,
    narrationFrequency,
    onNarrationFrequencyChange,
    textLength,
    onTextLengthChange,
    autoNarrate,
    onAutoNarrateChange,
    pauseDuration,
    onPauseDurationChange,
    repeatTTL,
    onRepeatTTLChange,
    narrationLengthShort,
    narrationLengthLong,
    onNarrationLengthChange,
    streamingMode,
    onStreamingModeChange,
    activeMapStyle,
    onActiveMapStyleChange,
    settlementLabelLimit,
    onSettlementLabelLimitChange,
    settlementTier: _settlementTier,
    onSettlementTierChange: _onSettlementTierChange,
    paperOpacityFog,
    onPaperOpacityFogChange,
    paperOpacityClear,
    onPaperOpacityClearChange,
    parchmentSaturation,
    onParchmentSaturationChange,
    showArtisticDebugBoxes,
    onShowArtisticDebugBoxesChange,
    volume,
    onVolumeChange,
    aircraftIcon,
    onAircraftIconChange,
    aircraftSize,
    onAircraftSizeChange,
    aircraftColorMain,
    onAircraftColorMainChange,
    aircraftColorAccent,
    onAircraftColorAccentChange,
}) => {
    const [activeTab, setActiveTab] = useState('narrator');
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);
    const [librariesExpanded, setLibrariesExpanded] = useState(false);

    // Server config (original values)
    const [serverConfig, setServerConfig] = useState<any>(null);

    // Draft state (local edits)
    const [draft, setDraft] = useState<DraftState | null>(null);

    // Load config from server
    useEffect(() => {
        fetch('/api/config')
            .then(r => r.json())
            .then(data => {
                setServerConfig(data);
                // Initialize draft from server + props
                setDraft({
                    narrationFrequency: data.narration_frequency ?? narrationFrequency,
                    textLength: data.text_length ?? textLength,
                    autoNarrate: data.auto_narrate ?? autoNarrate,
                    pauseDuration: data.pause_between_narrations ?? pauseDuration,
                    repeatTTL: data.repeat_ttl ?? repeatTTL,
                    repeatTTLInput: secondsToHuman(data.repeat_ttl ?? repeatTTL),
                    narrationLengthShort: data.narration_length_short_words ?? narrationLengthShort,
                    narrationLengthLong: data.narration_length_long_words ?? narrationLengthLong,
                    promptUnits: data.units || 'hybrid',
                    minPoiScore: data.min_poi_score ?? minPoiScore,
                    filterMode: data.filter_mode || filterMode,
                    targetPoiCount: data.target_poi_count ?? targetPoiCount,
                    activeStyle: data.active_style || '',
                    styleLibrary: data.style_library || [],
                    activeSecretWord: data.active_secret_word || '',
                    secretWordLibrary: data.secret_word_library || [],
                    activeTargetLanguage: data.active_target_language || 'en-US',
                    targetLanguageLibrary: data.target_language_library || ['en-US'],
                    simSource: data.sim_source || 'simconnect',
                    mockStartLat: data.mock_start_lat ?? null,
                    mockStartLon: data.mock_start_lon ?? null,
                    mockStartAlt: data.mock_start_alt ?? null,
                    mockStartHeading: data.mock_start_heading ?? null,
                    mockDurationParked: data.mock_duration_parked ?? 0,
                    mockDurationParkedInput: secondsToHuman(data.mock_duration_parked ?? 0),
                    mockDurationTaxi: data.mock_duration_taxi ?? 0,
                    mockDurationTaxiInput: secondsToHuman(data.mock_duration_taxi ?? 0),
                    mockDurationHold: data.mock_duration_hold ?? 0,
                    mockDurationHoldInput: secondsToHuman(data.mock_duration_hold ?? 0),
                    units: data.range_ring_units || units,
                    showCacheLayer: data.show_cache_layer ?? showCacheLayer,
                    showVisibilityLayer: data.show_visibility_layer ?? showVisibilityLayer,
                    activeMapStyle: data.active_map_style || activeMapStyle,
                    streamingMode,
                    deferralThreshold: data.deferral_threshold ?? 1.05,
                    deferralProximityBoostPower: data.deferral_proximity_boost_power ?? 1.0,
                    twoPassScriptGeneration: data.two_pass_script_generation ?? false,
                    beaconEnabled: data.beacon_enabled ?? true,
                    beaconFormationEnabled: data.beacon_formation_enabled ?? true,
                    beaconFormationDistance: data.beacon_formation_distance ?? 2000,
                    beaconFormationCount: data.beacon_formation_count ?? 3,
                    beaconMinSpawnAltitude: data.beacon_min_spawn_altitude ?? 304.8,
                    beaconAltitudeFloor: data.beacon_altitude_floor ?? 609.6,
                    beaconSinkDistanceFar: data.beacon_sink_distance_far ?? 5000,
                    beaconSinkDistanceClose: data.beacon_sink_distance_close ?? 2000,
                    beaconTargetFloorAGL: data.beacon_target_floor_agl ?? 30.48,

                    beaconMaxTargets: data.beacon_max_targets ?? 2,
                    settlementLabelLimit: data.settlement_label_limit ?? settlementLabelLimit,
                    settlementTier: data.settlement_tier ?? 0,
                    paperOpacityFog: data.paper_opacity_fog ?? paperOpacityFog,
                    paperOpacityClear: data.paper_opacity_clear ?? paperOpacityClear,
                    parchmentSaturation: data.parchment_saturation ?? parchmentSaturation,
                    showArtisticDebugBoxes: showArtisticDebugBoxes,
                    // Aircraft
                    aircraftIcon: (data.aircraft_icon as AircraftType) || (aircraftIcon as AircraftType) || 'balloon',
                    aircraftSize: data.aircraft_size || aircraftSize || 32,
                    aircraftColorMain: data.aircraft_color_main || aircraftColorMain || '#e63946',
                    aircraftColorAccent: data.aircraft_color_accent || aircraftColorAccent || '#ffffff',
                    // Audio
                    volume: data.volume ?? volume ?? 1.0,
                });
                setLoading(false);
            })
            .catch(e => console.error("Failed to fetch settings", e));
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    // Check if draft differs from original
    const hasChanges = () => {
        if (!draft || !serverConfig) return false;
        return (
            draft.narrationFrequency !== narrationFrequency ||
            draft.textLength !== textLength ||
            draft.autoNarrate !== autoNarrate ||
            draft.pauseDuration !== pauseDuration ||
            draft.repeatTTL !== repeatTTL ||
            draft.narrationLengthShort !== narrationLengthShort ||
            draft.narrationLengthLong !== narrationLengthLong ||
            draft.promptUnits !== (serverConfig.units || 'hybrid') ||
            draft.minPoiScore !== minPoiScore ||
            draft.filterMode !== filterMode ||
            draft.targetPoiCount !== targetPoiCount ||
            draft.activeStyle !== (serverConfig.active_style || '') ||
            JSON.stringify(draft.styleLibrary) !== JSON.stringify(serverConfig.style_library || []) ||
            draft.activeSecretWord !== (serverConfig.active_secret_word || '') ||
            JSON.stringify(draft.secretWordLibrary) !== JSON.stringify(serverConfig.secret_word_library || []) ||
            draft.activeTargetLanguage !== (serverConfig.active_target_language || 'en-US') ||
            JSON.stringify(draft.targetLanguageLibrary) !== JSON.stringify(serverConfig.target_language_library || ['en-US']) ||
            draft.simSource !== (serverConfig.sim_source || 'simconnect') ||
            draft.mockStartLat !== (serverConfig.mock_start_lat ?? null) ||
            draft.mockStartLon !== (serverConfig.mock_start_lon ?? null) ||
            draft.mockStartAlt !== (serverConfig.mock_start_alt ?? null) ||
            draft.mockStartHeading !== (serverConfig.mock_start_heading ?? null) ||
            draft.mockDurationParked !== (serverConfig.mock_duration_parked ?? 0) ||
            draft.mockDurationTaxi !== (serverConfig.mock_duration_taxi ?? 0) ||
            draft.mockDurationHold !== (serverConfig.mock_duration_hold ?? 0) ||
            draft.units !== units ||
            draft.showCacheLayer !== showCacheLayer ||
            draft.showVisibilityLayer !== showVisibilityLayer ||
            draft.activeMapStyle !== activeMapStyle ||
            draft.streamingMode !== streamingMode ||
            draft.deferralThreshold !== (serverConfig.deferral_threshold ?? 1.05) ||
            draft.twoPassScriptGeneration !== (serverConfig.two_pass_script_generation ?? false) ||
            draft.beaconEnabled !== (serverConfig.beacon_enabled ?? true) ||
            draft.beaconFormationEnabled !== (serverConfig.beacon_formation_enabled ?? true) ||
            draft.beaconFormationDistance !== (serverConfig.beacon_formation_distance ?? 2000) ||
            draft.beaconFormationCount !== (serverConfig.beacon_formation_count ?? 3) ||
            draft.beaconMinSpawnAltitude !== (serverConfig.beacon_min_spawn_altitude ?? 304.8) ||
            draft.beaconAltitudeFloor !== (serverConfig.beacon_altitude_floor ?? 609.6) ||
            draft.beaconSinkDistanceFar !== (serverConfig.beacon_sink_distance_far ?? 5000) ||
            draft.beaconSinkDistanceClose !== (serverConfig.beacon_sink_distance_close ?? 2000) ||
            draft.beaconTargetFloorAGL !== (serverConfig.beacon_target_floor_agl ?? 30.48) ||
            draft.beaconMaxTargets !== (serverConfig.beacon_max_targets ?? 2) ||
            draft.settlementLabelLimit !== settlementLabelLimit ||
            draft.settlementTier !== _settlementTier ||
            draft.paperOpacityFog !== paperOpacityFog ||
            draft.paperOpacityClear !== paperOpacityClear ||
            draft.parchmentSaturation !== parchmentSaturation ||
            draft.showArtisticDebugBoxes !== showArtisticDebugBoxes ||
            draft.aircraftColorMain !== (serverConfig.aircraft_color_main || aircraftColorMain || '#e63946') ||
            draft.aircraftColorAccent !== (serverConfig.aircraft_color_accent || aircraftColorAccent || '#ffffff') ||
            draft.volume !== (serverConfig.volume ?? volume ?? 1.0)
        );
    };

    // Update draft field
    const updateDraft = <K extends keyof DraftState>(key: K, value: DraftState[K]) => {
        setDraft(prev => prev ? { ...prev, [key]: value } : null);
    };

    // Save all changes
    const handleSave = async () => {
        if (!draft) return;
        setSaving(true);

        // Build payload with server-only fields (not handled by callbacks)
        const payload: Record<string, any> = {};

        if (draft.promptUnits !== (serverConfig?.units || 'hybrid')) payload.units = draft.promptUnits;
        if (draft.activeStyle !== (serverConfig?.active_style || '')) payload.active_style = draft.activeStyle;
        if (JSON.stringify(draft.styleLibrary) !== JSON.stringify(serverConfig?.style_library || [])) payload.style_library = draft.styleLibrary;
        if (draft.activeSecretWord !== (serverConfig?.active_secret_word || '')) payload.active_secret_word = draft.activeSecretWord;
        if (JSON.stringify(draft.secretWordLibrary) !== JSON.stringify(serverConfig?.secret_word_library || [])) payload.secret_word_library = draft.secretWordLibrary;
        if (draft.activeTargetLanguage !== (serverConfig?.active_target_language || 'en-US')) payload.active_target_language = draft.activeTargetLanguage;
        if (JSON.stringify(draft.targetLanguageLibrary) !== JSON.stringify(serverConfig?.target_language_library || ['en-US'])) payload.target_language_library = draft.targetLanguageLibrary;
        if (draft.simSource !== (serverConfig?.sim_source || 'simconnect')) payload.sim_source = draft.simSource;
        if (draft.mockStartLat !== (serverConfig?.mock_start_lat ?? null)) payload.mock_start_lat = draft.mockStartLat;
        if (draft.mockStartLon !== (serverConfig?.mock_start_lon ?? null)) payload.mock_start_lon = draft.mockStartLon;
        if (draft.mockStartAlt !== (serverConfig?.mock_start_alt ?? null)) payload.mock_start_alt = draft.mockStartAlt;
        if (draft.mockStartHeading !== (serverConfig?.mock_start_heading ?? null)) payload.mock_start_heading = draft.mockStartHeading;
        if (draft.mockDurationParked !== (serverConfig?.mock_duration_parked ?? 0)) payload.mock_duration_parked = draft.mockDurationParked;
        if (draft.mockDurationTaxi !== (serverConfig?.mock_duration_taxi ?? 0)) payload.mock_duration_taxi = draft.mockDurationTaxi;
        if (draft.mockDurationHold !== (serverConfig?.mock_duration_hold ?? 0)) payload.mock_duration_hold = draft.mockDurationHold;
        if (draft.deferralThreshold !== (serverConfig?.deferral_threshold ?? 1.05)) payload.deferral_threshold = draft.deferralThreshold;
        if (draft.deferralProximityBoostPower !== (serverConfig?.deferral_proximity_boost_power ?? 1.0)) payload.deferral_proximity_boost_power = draft.deferralProximityBoostPower;
        if (draft.twoPassScriptGeneration !== (serverConfig?.two_pass_script_generation ?? false)) payload.two_pass_script_generation = draft.twoPassScriptGeneration;
        if (draft.autoNarrate !== (serverConfig?.auto_narrate ?? true)) payload.auto_narrate = draft.autoNarrate;
        if (draft.pauseDuration !== (serverConfig?.pause_between_narrations ?? 4)) payload.pause_between_narrations = draft.pauseDuration;
        if (draft.repeatTTL !== (serverConfig?.repeat_ttl ?? 3600)) payload.repeat_ttl = draft.repeatTTL;
        if (draft.narrationLengthShort !== (serverConfig?.narration_length_short_words ?? 50)) payload.narration_length_short_words = draft.narrationLengthShort;
        if (draft.narrationLengthLong !== (serverConfig?.narration_length_long_words ?? 200)) payload.narration_length_long_words = draft.narrationLengthLong;
        if (draft.beaconEnabled !== (serverConfig?.beacon_enabled ?? true)) payload.beacon_enabled = draft.beaconEnabled;
        if (draft.beaconFormationEnabled !== (serverConfig?.beacon_formation_enabled ?? true)) payload.beacon_formation_enabled = draft.beaconFormationEnabled;
        if (draft.beaconFormationDistance !== (serverConfig?.beacon_formation_distance ?? 2000)) payload.beacon_formation_distance = draft.beaconFormationDistance;
        if (draft.beaconFormationCount !== (serverConfig?.beacon_formation_count ?? 3)) payload.beacon_formation_count = draft.beaconFormationCount;
        if (draft.beaconMinSpawnAltitude !== (serverConfig?.beacon_min_spawn_altitude ?? 304.8)) payload.beacon_min_spawn_altitude = draft.beaconMinSpawnAltitude;
        if (draft.beaconAltitudeFloor !== (serverConfig?.beacon_altitude_floor ?? 609.6)) payload.beacon_altitude_floor = draft.beaconAltitudeFloor;
        if (draft.beaconSinkDistanceFar !== (serverConfig?.beacon_sink_distance_far ?? 5000)) payload.beacon_sink_distance_far = draft.beaconSinkDistanceFar;
        if (draft.beaconSinkDistanceClose !== (serverConfig?.beacon_sink_distance_close ?? 2000)) payload.beacon_sink_distance_close = draft.beaconSinkDistanceClose;
        if (draft.beaconTargetFloorAGL !== (serverConfig?.beacon_target_floor_agl ?? 30.48)) payload.beacon_target_floor_agl = draft.beaconTargetFloorAGL;
        if (draft.beaconMaxTargets !== (serverConfig?.beacon_max_targets ?? 2)) payload.beacon_max_targets = draft.beaconMaxTargets;
        // Aircraft
        if (draft.aircraftIcon !== (serverConfig?.aircraft_icon || 'balloon')) payload.aircraft_icon = draft.aircraftIcon;
        if (draft.aircraftSize !== (serverConfig?.aircraft_size || 32)) payload.aircraft_size = draft.aircraftSize;
        if (draft.aircraftColorMain !== (serverConfig?.aircraft_color_main || aircraftColorMain || '#e63946')) payload.aircraft_color_main = draft.aircraftColorMain;
        if (draft.aircraftColorAccent !== (serverConfig?.aircraft_color_accent || aircraftColorAccent || '#ffffff')) payload.aircraft_color_accent = draft.aircraftColorAccent;
        if (draft.volume !== (serverConfig?.volume ?? volume ?? 1.0)) payload.volume = draft.volume;

        try {
            // Send server-only changes
            if (Object.keys(payload).length > 0) {
                await fetch('/api/config', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload)
                });
            }

            // Apply local-only changes via callbacks (UI state only)
            if (draft.units !== units) onUnitsChange(draft.units);
            if (draft.showCacheLayer !== showCacheLayer) onCacheLayerChange(draft.showCacheLayer);
            if (draft.showVisibilityLayer !== showVisibilityLayer) onVisibilityLayerChange(draft.showVisibilityLayer);
            if (draft.activeMapStyle !== activeMapStyle) onActiveMapStyleChange(draft.activeMapStyle);
            if (draft.streamingMode !== streamingMode) onStreamingModeChange(draft.streamingMode);

            // Apply prop-based changes via callbacks (these update parent state AND send to server)
            if (draft.narrationFrequency !== narrationFrequency) onNarrationFrequencyChange(draft.narrationFrequency);
            if (draft.textLength !== textLength) onTextLengthChange(draft.textLength);
            if (draft.autoNarrate !== autoNarrate) onAutoNarrateChange(draft.autoNarrate);
            if (draft.pauseDuration !== pauseDuration) onPauseDurationChange(draft.pauseDuration);
            if (draft.repeatTTL !== repeatTTL) onRepeatTTLChange(draft.repeatTTL);
            if (draft.narrationLengthShort !== narrationLengthShort || draft.narrationLengthLong !== narrationLengthLong) {
                onNarrationLengthChange(draft.narrationLengthShort, draft.narrationLengthLong);
            }
            if (draft.minPoiScore !== minPoiScore) onMinPoiScoreChange(draft.minPoiScore);
            if (draft.filterMode !== filterMode) onFilterModeChange(draft.filterMode);
            if (draft.targetPoiCount !== targetPoiCount) onTargetPoiCountChange(draft.targetPoiCount);
            if (draft.settlementLabelLimit !== settlementLabelLimit) onSettlementLabelLimitChange(draft.settlementLabelLimit);
            if (draft.settlementTier !== _settlementTier) _onSettlementTierChange(draft.settlementTier);
            if (draft.paperOpacityFog !== paperOpacityFog) onPaperOpacityFogChange(draft.paperOpacityFog);
            if (draft.paperOpacityClear !== paperOpacityClear) onPaperOpacityClearChange(draft.paperOpacityClear);
            if (draft.parchmentSaturation !== parchmentSaturation) onParchmentSaturationChange(draft.parchmentSaturation);
            if (draft.showArtisticDebugBoxes !== showArtisticDebugBoxes) onShowArtisticDebugBoxesChange(draft.showArtisticDebugBoxes);
            if (draft.volume !== volume) onVolumeChange(draft.volume);
            if (draft.aircraftIcon !== aircraftIcon) onAircraftIconChange(draft.aircraftIcon);
            if (draft.aircraftSize !== aircraftSize) onAircraftSizeChange(draft.aircraftSize);
            if (draft.aircraftColorMain !== aircraftColorMain) onAircraftColorMainChange(draft.aircraftColorMain);
            if (draft.aircraftColorAccent !== aircraftColorAccent) onAircraftColorAccentChange(draft.aircraftColorAccent);

            // Update server config to match saved values
            setServerConfig((prev: any) => ({
                ...prev,
                ...payload
            }));

            // Close dialog after successful save
            onBack();
        } catch (e) {
            console.error("Failed to save settings", e);
        } finally {
            setSaving(false);
        }
    };

    // Discard changes and close dialog
    const handleDiscard = () => {
        onBack();
    };

    const renderField = (label: string, field: React.ReactNode, restart = false) => (
        <div className="settings-field">
            <div className="settings-label-row">
                <span className="role-label">{label}{restart && ' *'}</span>
            </div>
            {field}
        </div>
    );

    const tabs = [
        { id: 'narrator', label: 'Narrator' },
        { id: 'sim', label: 'Simulator' },
        { id: 'beacon', label: 'Beacons' },
        { id: 'scorer', label: 'Scorer' },
        { id: 'interface', label: 'Interface' },
        { id: 'debug', label: 'Debugging' }
    ];

    if (loading || !draft) {
        return (
            <div className="settings-overlay">
                <div className="settings-loading">Consulting the Archives...</div>
            </div>
        );
    }

    const changed = hasChanges();

    return (
        <div className="settings-overlay">
            <div className="settings-container">
                <div className="settings-sidebar">
                    <div className="settings-branding">
                        <div className="role-title">Phileas</div>
                        <div className="role-header" style={{ fontSize: '12px' }}>Configuration</div>
                    </div>
                    <div className="settings-nav">
                        {tabs.map(tab => (
                            <div
                                key={tab.id}
                                className={`settings-tab ${activeTab === tab.id ? 'active' : ''}`}
                                onClick={() => setActiveTab(tab.id)}
                            >
                                {tab.label}
                            </div>
                        ))}
                    </div>
                    <div className="settings-actions">
                        <button
                            className={`settings-save-btn ${changed ? 'has-changes' : ''}`}
                            onClick={handleSave}
                            disabled={!changed || saving}
                        >
                            {saving ? 'Saving...' : 'Save Changes'}
                        </button>
                        <button
                            className="settings-discard-btn"
                            onClick={handleDiscard}
                            disabled={saving}
                        >
                            {changed ? 'Discard' : 'Close'}
                        </button>
                    </div>
                    <div className="settings-footer">
                        * Requires application restart
                    </div>
                </div>

                <div className="settings-content">
                    {activeTab === 'sim' && (
                        <div className="settings-group">
                            <div className="role-header">Source & Connection</div>
                            {renderField('Simulator Provider', (
                                <select
                                    className="settings-select"
                                    value={draft.simSource}
                                    onChange={e => updateDraft('simSource', e.target.value)}
                                >
                                    <option value="simconnect">Microsoft Flight Simulator</option>
                                    <option value="mock">Mock Simulator (Internal)</option>
                                </select>
                            ), true)}

                            {draft.simSource === 'mock' && (
                                <>
                                    <div className="role-header" style={{ marginTop: '24px' }}>Mock Simulator Parameters</div>
                                    <div className="settings-grid">
                                        {renderField('Start Latitude', (
                                            <input
                                                type="number"
                                                className="settings-input"
                                                value={draft.mockStartLat ?? ''}
                                                onChange={e => updateDraft('mockStartLat', e.target.value === '' ? null : parseFloat(e.target.value))}
                                            />
                                        ))}
                                        {renderField('Start Longitude', (
                                            <input
                                                type="number"
                                                className="settings-input"
                                                value={draft.mockStartLon ?? ''}
                                                onChange={e => updateDraft('mockStartLon', e.target.value === '' ? null : parseFloat(e.target.value))}
                                            />
                                        ))}
                                        {renderField('Start Altitude (ft)', (
                                            <input
                                                type="number"
                                                className="settings-input"
                                                value={draft.mockStartAlt ?? ''}
                                                onChange={e => updateDraft('mockStartAlt', e.target.value === '' ? null : parseFloat(e.target.value))}
                                            />
                                        ))}
                                        {renderField('Start Heading (deg)', (
                                            <input
                                                type="number"
                                                className="settings-input"
                                                placeholder="Automatic"
                                                value={draft.mockStartHeading ?? ''}
                                                onChange={e => updateDraft('mockStartHeading', e.target.value === '' ? null : parseFloat(e.target.value))}
                                            />
                                        ))}
                                    </div>
                                    <div className="settings-grid">
                                        {renderField('Parked Duration', (
                                            <input
                                                type="text"
                                                className="settings-input"
                                                placeholder="e.g. 30s, 1m"
                                                value={draft.mockDurationParkedInput}
                                                onChange={e => {
                                                    const input = e.target.value;
                                                    updateDraft('mockDurationParkedInput', input);
                                                    const secs = parseHumanToSeconds(input);
                                                    if (secs !== null) updateDraft('mockDurationParked', secs);
                                                }}
                                            />
                                        ))}
                                        {renderField('Taxi Duration', (
                                            <input
                                                type="text"
                                                className="settings-input"
                                                placeholder="e.g. 2m"
                                                value={draft.mockDurationTaxiInput}
                                                onChange={e => {
                                                    const input = e.target.value;
                                                    updateDraft('mockDurationTaxiInput', input);
                                                    const secs = parseHumanToSeconds(input);
                                                    if (secs !== null) updateDraft('mockDurationTaxi', secs);
                                                }}
                                            />
                                        ))}
                                        {renderField('Hold Duration', (
                                            <input
                                                type="text"
                                                className="settings-input"
                                                placeholder="e.g. 10s"
                                                value={draft.mockDurationHoldInput}
                                                onChange={e => {
                                                    const input = e.target.value;
                                                    updateDraft('mockDurationHoldInput', input);
                                                    const secs = parseHumanToSeconds(input);
                                                    if (secs !== null) updateDraft('mockDurationHold', secs);
                                                }}
                                            />
                                        ))}
                                    </div>
                                </>
                            )}
                        </div>
                    )}

                    {activeTab === 'narrator' && (
                        <div className="settings-group">
                            <div className="role-header">Narration Preferences</div>
                            <VictorianToggle
                                label="Automatic Narration"
                                checked={draft.autoNarrate}
                                onChange={val => updateDraft('autoNarrate', val)}
                            />
                            <VictorianToggle
                                label="2-pass script generation"
                                checked={draft.twoPassScriptGeneration}
                                onChange={val => updateDraft('twoPassScriptGeneration', val)}
                            />
                            {renderField('Master Volume', (
                                <div className="settings-slider-container">
                                    <span className="role-value">{Math.round(draft.volume * 100)}%</span>
                                    <input
                                        type="range"
                                        min="0" max="1" step="0.01"
                                        value={draft.volume}
                                        onChange={e => updateDraft('volume', parseFloat(e.target.value))}
                                    />
                                </div>
                            ))}
                            {renderField('Pause Between Narrations', (
                                <div className="settings-slider-container">
                                    <span className="role-value">{draft.pauseDuration}s</span>
                                    <input
                                        type="range"
                                        min="1" max="10"
                                        value={draft.pauseDuration}
                                        onChange={e => updateDraft('pauseDuration', parseInt(e.target.value))}
                                    />
                                </div>
                            ))}
                            {renderField('Repeat TTL', (
                                <input
                                    type="text"
                                    className="settings-input"
                                    placeholder="e.g. 1h, 30m, 1d"
                                    value={draft.repeatTTLInput}
                                    onChange={e => {
                                        const input = e.target.value;
                                        updateDraft('repeatTTLInput', input);
                                        const secs = parseHumanToSeconds(input);
                                        if (secs !== null) updateDraft('repeatTTL', secs);
                                    }}
                                />
                            ))}
                            <div style={{ marginTop: '16px' }}></div>
                            {renderField('Narration Frequency', (
                                <div className="settings-slider-container">
                                    <span className="role-value">{['Rarely', 'Normal', 'Active', 'Hyperactive'][draft.narrationFrequency - 1] || draft.narrationFrequency}</span>
                                    <input type="range" min="1" max="4" value={draft.narrationFrequency} onChange={e => updateDraft('narrationFrequency', parseInt(e.target.value))} />
                                </div>
                            ))}
                            {renderField('Script Length', (
                                <div className="settings-slider-container">
                                    <span className="role-value">{['Short', 'Brief', 'Normal', 'Detailed', 'Long'][draft.textLength - 1] || draft.textLength}</span>
                                    <input type="range" min="1" max="5" value={draft.textLength} onChange={e => updateDraft('textLength', parseInt(e.target.value))} />
                                </div>
                            ))}
                            {renderField('Units', (
                                <select className="settings-select" value={draft.promptUnits} onChange={e => updateDraft('promptUnits', e.target.value)}>
                                    <option value="imperial">Imperial (ft, knots, °F)</option>
                                    <option value="hybrid">Hybrid (m, knots, °C)</option>
                                    <option value="metric">Metric (m, km/h, °C)</option>
                                </select>
                            ))}

                            <div className="role-header" style={{ marginTop: '24px' }}>Language</div>
                            {renderField('Active Language', (
                                <select
                                    className="settings-select"
                                    value={draft.activeTargetLanguage}
                                    onChange={e => updateDraft('activeTargetLanguage', e.target.value)}
                                >
                                    {draft.targetLanguageLibrary.map((lang: string) => (
                                        <option key={lang} value={lang}>{lang}</option>
                                    ))}
                                </select>
                            ))}

                            {librariesExpanded && renderField('Language Library', (
                                <VictorianListEditor
                                    values={draft.targetLanguageLibrary}
                                    placeholder="Add language (e.g. de-DE)..."
                                    onChange={val => updateDraft('targetLanguageLibrary', val)}
                                />
                            ))}

                            <div className="role-header" style={{ marginTop: '24px' }}>POI Selection</div>
                            {renderField('POI Filtering Mode', (
                                <select className="settings-select" value={draft.filterMode} onChange={e => updateDraft('filterMode', e.target.value)}>
                                    <option value="fixed">Fixed Score</option>
                                    <option value="adaptive">Adaptive Score</option>
                                </select>
                            ))}
                            {draft.filterMode === 'adaptive' ?
                                renderField('Target POI Count', (
                                    <div className="settings-slider-container">
                                        <span className="role-value">{draft.targetPoiCount}</span>
                                        <input type="range" min="5" max="50" step="5" value={draft.targetPoiCount} onChange={e => updateDraft('targetPoiCount', parseInt(e.target.value))} />
                                    </div>
                                )) :
                                renderField('Score Threshold', (
                                    <div className="settings-slider-container">
                                        <span className="role-value">{draft.minPoiScore.toFixed(1)}</span>
                                        <input
                                            type="range"
                                            min="-10" max="10" step="0.5"
                                            value={draft.minPoiScore}
                                            onChange={e => updateDraft('minPoiScore', parseFloat(e.target.value))}
                                        />
                                    </div>
                                ))
                            }

                            <div className="role-header" style={{ marginTop: '24px' }}>Visibility & Deferral</div>
                            {renderField('Proximity Boost Power', (
                                <div className="settings-slider-container">
                                    <span className="role-value">x{draft.deferralProximityBoostPower.toFixed(1)}</span>
                                    <input
                                        type="range"
                                        min="1.0" max="4.0" step="0.1"
                                        value={draft.deferralProximityBoostPower}
                                        onChange={e => updateDraft('deferralProximityBoostPower', parseFloat(e.target.value))}
                                    />
                                </div>
                            ))}
                            <div className="settings-footer" style={{ marginTop: '12px', fontSize: '12px', color: 'var(--muted)', fontStyle: 'normal' }}>
                                Higher values prioritize perfect viewing moments over immediate narration by punishing distance more heavily.
                            </div>

                            <div style={{ marginTop: '24px' }}></div>
                            {renderField('Wait for better view', (
                                <div className="settings-slider-container">
                                    <span className="role-value">{Math.round((draft.deferralThreshold - 1.0) * 100)}%</span>
                                    <input
                                        type="range"
                                        min="0" max="20" step="1"
                                        value={Math.round((draft.deferralThreshold - 1.0) * 100)}
                                        onChange={e => updateDraft('deferralThreshold', 1.0 + (parseInt(e.target.value) / 100.0))}
                                    />
                                </div>
                            ))}
                            <div className="settings-footer" style={{ marginTop: '12px', fontSize: '12px', color: 'var(--muted)', fontStyle: 'normal' }}>
                                Minimum visual improvement required to defer narration. At 0%, Phileas always waits for the absolute peak view.
                            </div>

                            <div className="role-header" style={{ marginTop: '24px' }}>Style Library</div>
                            {renderField('Active Style Influence', (
                                <select
                                    className="settings-select"
                                    value={draft.activeStyle}
                                    onChange={e => updateDraft('activeStyle', e.target.value)}
                                >
                                    <option value="">(Standard Narration)</option>
                                    {draft.styleLibrary.map((s: string) => (
                                        <option key={s} value={s}>{s}</option>
                                    ))}
                                </select>
                            ))}

                            {librariesExpanded && renderField('Style Library', (
                                <VictorianListEditor
                                    values={draft.styleLibrary}
                                    placeholder="Add author to library..."
                                    onChange={val => updateDraft('styleLibrary', val)}
                                />
                            ))}

                            <div className="role-header" style={{ marginTop: '24px' }}>Trip Theme</div>
                            {renderField('Active Theme', (
                                <select
                                    className="settings-select"
                                    value={draft.activeSecretWord}
                                    onChange={e => updateDraft('activeSecretWord', e.target.value)}
                                >
                                    <option value="">(No Theme)</option>
                                    {draft.secretWordLibrary.map((s: string) => (
                                        <option key={s} value={s}>{s}</option>
                                    ))}
                                </select>
                            ))}

                            {librariesExpanded && renderField('Theme Library', (
                                <VictorianListEditor
                                    values={draft.secretWordLibrary}
                                    placeholder="Add theme to library..."
                                    onChange={val => updateDraft('secretWordLibrary', val)}
                                />
                            ))}

                            {renderField('Narration Word Targets', (
                                <DualRangeSlider
                                    min={10}
                                    max={1000}
                                    step={10}
                                    minVal={draft.narrationLengthShort}
                                    maxVal={draft.narrationLengthLong}
                                    unit=" words"
                                    onChange={(min, max) => {
                                        updateDraft('narrationLengthShort', min);
                                        updateDraft('narrationLengthLong', max);
                                    }}
                                />
                            ))}

                            <div
                                className="role-header"
                                style={{ marginTop: '24px', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '8px' }}
                                onClick={() => setLibrariesExpanded(!librariesExpanded)}
                            >
                                <span style={{ fontSize: '10px', opacity: 0.6 }}>{librariesExpanded ? '▼' : '▶'}</span>
                                Library Management
                            </div>
                        </div>
                    )}

                    {activeTab === 'scorer' && (
                        <div className="settings-group">
                            <div className="role-header">Aircraft Customization</div>

                            {/* Live Preview Card */}
                            <div style={{
                                background: '#f4ecd8',
                                border: '1px solid #d4c5a3',
                                borderRadius: '8px',
                                padding: '16px',
                                marginBottom: '24px',
                                display: 'flex',
                                flexDirection: 'column',
                                alignItems: 'center',
                                gap: '8px',
                                position: 'relative',
                                overflow: 'hidden'
                            }}>
                                <div style={{
                                    position: 'absolute',
                                    top: 0, left: 0, right: 0, bottom: 0,
                                    backgroundImage: 'url("https://watercolormaps.collection.cooperhewitt.org/tile/watercolor/12/2154/1363.jpg")', // Sample tile
                                    backgroundSize: 'cover',
                                    opacity: 0.5,
                                    pointerEvents: 'none'
                                }} />
                                <div style={{ position: 'relative', width: '128px', height: '128px', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                                    <AircraftIcon
                                        type={draft.aircraftIcon}
                                        x={64} y={64}
                                        agl={5000}
                                        heading={45}
                                        size={draft.aircraftSize} // Live size update
                                        colorMain={draft.aircraftColorMain}
                                        colorAccent={draft.aircraftColorAccent}
                                    />
                                </div>
                                <span className="role-label" style={{ zIndex: 1 }}>Live Preview</span>
                            </div>

                            {/* Icon Type Selection */}
                            {renderField('Aircraft Type', (
                                <div className="settings-grid" style={{ gridTemplateColumns: 'repeat(3, 1fr)', gap: '8px' }}>
                                    {(['balloon', 'prop', 'twin_prop', 'jet', 'airliner', 'helicopter'] as AircraftType[]).map(type => (
                                        <div
                                            key={type}
                                            onClick={() => updateDraft('aircraftIcon', type)}
                                            style={{
                                                border: draft.aircraftIcon === type ? '2px solid #5c4033' : '1px solid #d4c5a3',
                                                borderRadius: '6px',
                                                padding: '8px',
                                                cursor: 'pointer',
                                                background: draft.aircraftIcon === type ? 'rgba(92, 64, 51, 0.1)' : 'transparent',
                                                display: 'flex',
                                                flexDirection: 'column',
                                                alignItems: 'center',
                                                gap: '4px'
                                            }}
                                        >
                                            <div style={{ width: '32px', height: '32px', position: 'relative' }}>
                                                <AircraftIcon
                                                    type={type}
                                                    x={16} y={16} agl={0} heading={0}
                                                    size={24}
                                                    colorMain={draft.aircraftIcon === type ? draft.aircraftColorMain : '#666'}
                                                    colorAccent={draft.aircraftIcon === type ? draft.aircraftColorAccent : '#999'}
                                                />
                                            </div>
                                            <span style={{ fontSize: '10px', textTransform: 'capitalize' }}>{type.replace('_', ' ')}</span>
                                        </div>
                                    ))}
                                </div>
                            ))}

                            {/* Size Slider */}
                            {renderField('Icon Size', (
                                <div className="settings-slider-container">
                                    <span className="role-value">{draft.aircraftSize}px</span>
                                    <input
                                        type="range"
                                        min="16" max="64" step="4"
                                        value={draft.aircraftSize}
                                        onChange={e => updateDraft('aircraftSize', parseInt(e.target.value))}
                                    />
                                </div>
                            ))}

                            <div className="role-header" style={{ marginTop: '24px' }}>Livery Colors</div>

                            {/* Main Color Picker */}
                            {renderField('Main Color', (
                                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(8, 1fr)', gap: '4px' }}>
                                    {VICTORIAN_PALETTE.map(c => (
                                        <div
                                            key={c}
                                            onClick={() => updateDraft('aircraftColorMain', c)}
                                            style={{
                                                width: '100%',
                                                paddingBottom: '100%',
                                                backgroundColor: c,
                                                cursor: 'pointer',
                                                border: draft.aircraftColorMain === c ? '2px solid white' : '1px solid rgba(0,0,0,0.1)',
                                                borderRadius: '2px',
                                                boxShadow: draft.aircraftColorMain === c ? '0 0 0 2px #333' : 'none'
                                            }}
                                            title={c}
                                        />
                                    ))}
                                </div>
                            ))}

                            {/* Accent Color Picker */}
                            {renderField('Accent Color', (
                                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(8, 1fr)', gap: '4px' }}>
                                    {VICTORIAN_PALETTE.map(c => (
                                        <div
                                            key={c}
                                            onClick={() => updateDraft('aircraftColorAccent', c)}
                                            style={{
                                                width: '100%',
                                                paddingBottom: '100%',
                                                backgroundColor: c,
                                                cursor: 'pointer',
                                                border: draft.aircraftColorAccent === c ? '2px solid white' : '1px solid rgba(0,0,0,0.1)',
                                                borderRadius: '2px',
                                                boxShadow: draft.aircraftColorAccent === c ? '0 0 0 2px #333' : 'none'
                                            }}
                                            title={c}
                                        />
                                    ))}
                                </div>
                            ))}

                            <div style={{ marginTop: '24px', borderTop: '1px solid rgba(0,0,0,0.1)', paddingTop: '16px' }}></div>

                            {/* Scoring Parameters moved to Narrator tab - duplicate removed */}
                        </div>
                    )}

                    {activeTab === 'interface' && (
                        <div className="settings-group">
                            <div className="role-header">Use Map Style</div>
                            {renderField('Map Style', (
                                <select className="settings-select" value={draft.activeMapStyle} onChange={e => updateDraft('activeMapStyle', e.target.value)}>
                                    <option value="dark">Dark</option>
                                    <option value="artistic">Artistic</option>
                                </select>
                            ))}

                            {draft.activeMapStyle === 'artistic' && (
                                <>
                                    <div className="role-header" style={{ marginTop: '24px' }}>Map Labels</div>
                                    {renderField('Settlement labels', (
                                        <div className="settings-slider-container">
                                            <span className="role-value">
                                                {draft.settlementLabelLimit === -1 ? 'infinite' : draft.settlementLabelLimit}
                                            </span>
                                            <input
                                                type="range"
                                                min="0" max="21" step="1"
                                                value={draft.settlementLabelLimit === -1 ? 21 : draft.settlementLabelLimit}
                                                onChange={e => {
                                                    const val = parseInt(e.target.value);
                                                    updateDraft('settlementLabelLimit', val === 21 ? -1 : val);
                                                }}
                                            />
                                        </div>
                                    ))}
                                    {renderField('Settlement Size', (
                                        <div className="settings-slider-container">
                                            <span className="role-value">
                                                {draft.settlementTier === 0 ? 'None' :
                                                    draft.settlementTier === 1 ? 'City' :
                                                        draft.settlementTier === 2 ? 'Town' : 'Village'}
                                            </span>
                                            <input
                                                type="range"
                                                min="0" max="3" step="1"
                                                value={draft.settlementTier}
                                                onChange={e => updateDraft('settlementTier', parseInt(e.target.value))}
                                            />
                                        </div>
                                    ))}

                                    <div className="role-header" style={{ marginTop: '24px' }}>Overlay Layers</div>
                                    {renderField('Paper Opacity (Inside / Outside)', (
                                        <DualRangeSlider
                                            min={0}
                                            max={100}
                                            step={1}
                                            minVal={Math.round(draft.paperOpacityClear * 100)}
                                            maxVal={Math.round(draft.paperOpacityFog * 100)}
                                            unit="%"
                                            onChange={(min, max) => {
                                                updateDraft('paperOpacityClear', min / 100);
                                                updateDraft('paperOpacityFog', max / 100);
                                            }}
                                        />
                                    ))}
                                    {renderField('Paper Saturation', (
                                        <div className="settings-slider-container">
                                            <span className="role-value">{(draft.parchmentSaturation * 100).toFixed(0)}%</span>
                                            <input
                                                type="range"
                                                min="0" max="200" step="1"
                                                value={Math.round(draft.parchmentSaturation * 100)}
                                                onChange={e => updateDraft('parchmentSaturation', parseInt(e.target.value) / 100)}
                                            />
                                        </div>
                                    ))}

                                    <div className="role-header" style={{ marginTop: '24px' }}>Legal & Attribution</div>
                                    <div className="settings-footer" style={{ marginTop: '8px', fontSize: '11px', color: 'var(--muted)', fontStyle: 'normal', lineHeight: '1.4' }}>
                                        <strong>Map Tiles:</strong> Stamen Design (Watercolor), CartoDB (Labels/Dark).<br />
                                        <strong>Data:</strong> OpenStreetMap contributors (ODbL).<br />
                                        <strong>Icons:</strong> Lucide React, FontAwesome.<br />
                                        <strong>Fonts:</strong> IM Fell DW Pica (Igino Marini).
                                    </div>
                                </>
                            )}

                            {draft.activeMapStyle !== 'artistic' && (
                                <>
                                    <div className="role-header" style={{ marginTop: '24px' }}>Units & Display</div>
                                    {renderField('Range Ring Units', (
                                        <select className="settings-select" value={draft.units} onChange={e => updateDraft('units', e.target.value as 'km' | 'nm')}>
                                            <option value="km">km</option>
                                            <option value="nm">nm</option>
                                        </select>
                                    ))}

                                    <div className="role-header" style={{ marginTop: '24px' }}>Debug Layers</div>
                                    <VictorianToggle
                                        label="Show Cache Layer"
                                        checked={draft.showCacheLayer}
                                        onChange={val => updateDraft('showCacheLayer', val)}
                                    />
                                    <VictorianToggle
                                        label="Show Visibility Layer"
                                        checked={draft.showVisibilityLayer}
                                        onChange={val => updateDraft('showVisibilityLayer', val)}
                                    />
                                </>
                            )}

                            <div className="role-header" style={{ marginTop: '24px' }}>Developer Settings</div>
                            <VictorianToggle
                                label="Streaming Mode (LocalStorage)"
                                checked={draft.streamingMode}
                                onChange={val => updateDraft('streamingMode', val)}
                            />
                        </div>
                    )}
                    {/* Visibility & Deferral moved to Narrator tab */}
                    {activeTab === 'beacon' && (
                        <div className="settings-group">
                            <div className="role-header">Target Beacons</div>
                            <VictorianToggle
                                label="Enable Visual Beacons"
                                checked={draft.beaconEnabled}
                                onChange={val => updateDraft('beaconEnabled', val)}
                            />

                            <div className="role-header" style={{ marginTop: '24px' }}>Formation Settings</div>
                            <VictorianToggle
                                label="Formation Companions"
                                checked={draft.beaconFormationEnabled}
                                onChange={val => updateDraft('beaconFormationEnabled', val)}
                            />
                            {renderField('Companion Distance', (
                                <div className="settings-slider-container">
                                    <span className="role-value">{draft.beaconFormationDistance}m</span>
                                    <input
                                        type="range"
                                        min="500" max="10000" step="100"
                                        value={draft.beaconFormationDistance}
                                        onChange={e => updateDraft('beaconFormationDistance', parseInt(e.target.value))}
                                    />
                                </div>
                            ))}
                            {renderField('Companion Count', (
                                <div className="settings-slider-container">
                                    <span className="role-value">{draft.beaconFormationCount}</span>
                                    <input
                                        type="range"
                                        min="1" max="5"
                                        value={draft.beaconFormationCount}
                                        onChange={e => updateDraft('beaconFormationCount', parseInt(e.target.value))}
                                    />
                                </div>
                            ))}

                            <div className="role-header" style={{ marginTop: '24px' }}>Altitude & Sinking</div>
                            <div className="settings-grid">
                                {renderField('Min Spawn (ft)', (
                                    <input
                                        type="number"
                                        className="settings-input"
                                        value={Math.round(draft.beaconMinSpawnAltitude * 3.28084)}
                                        onChange={e => updateDraft('beaconMinSpawnAltitude', parseFloat(e.target.value) / 3.28084)}
                                    />
                                ))}
                                {renderField('Altitude Floor (ft)', (
                                    <input
                                        type="number"
                                        className="settings-input"
                                        value={Math.round(draft.beaconAltitudeFloor * 3.28084)}
                                        onChange={e => updateDraft('beaconAltitudeFloor', parseFloat(e.target.value) / 3.28084)}
                                    />
                                ))}
                            </div>
                            {renderField('Target Ground Floor (ft)', (
                                <input
                                    type="number"
                                    className="settings-input"
                                    value={Math.round(draft.beaconTargetFloorAGL * 3.28084)}
                                    onChange={e => updateDraft('beaconTargetFloorAGL', parseFloat(e.target.value) / 3.28084)}
                                />
                            ))}

                            <div className="role-header" style={{ marginTop: '24px' }}>Behavior & Limits</div>
                            {renderField('Target Sink Distances (Close/Far)', (
                                <DualRangeSlider
                                    min={500}
                                    max={10000}
                                    step={100}
                                    minVal={draft.beaconSinkDistanceClose}
                                    maxVal={draft.beaconSinkDistanceFar}
                                    onChange={(minVal, maxVal) => {
                                        updateDraft('beaconSinkDistanceClose', minVal);
                                        updateDraft('beaconSinkDistanceFar', maxVal);
                                    }}
                                />
                            ))}
                            {renderField('Max Active Beacons', (
                                <div className="settings-slider-container">
                                    <span className="role-value">{draft.beaconMaxTargets}</span>
                                    <input
                                        type="range"
                                        min="1" max="20"
                                        value={draft.beaconMaxTargets}
                                        onChange={e => updateDraft('beaconMaxTargets', parseInt(e.target.value))}
                                    />
                                </div>
                            ))}
                        </div>
                    )}
                    {activeTab === 'debug' && (
                        <div className="settings-group">
                            <div className="role-header">Artistic Map</div>
                            <VictorianToggle
                                label="Show Bounding Boxes"
                                checked={draft.showArtisticDebugBoxes}
                                onChange={val => updateDraft('showArtisticDebugBoxes', val)}
                            />
                            <div className="settings-footer" style={{ marginTop: '12px', fontSize: '12px', color: 'var(--muted)', fontStyle: 'normal' }}>
                                Renders R-tree collision bounding boxes for the placement engine. Red = marker, blue = label.
                            </div>
                        </div>
                    )}
                </div>
            </div>

            <style>{`
    .settings-overlay {
    position: fixed;
    top: 0; left: 0; right: 0; bottom: 0;
    background: var(--bg-color);
    z-index: 5000;
    display: flex;
    font-family: var(--font-main);
    }

    .settings-container {
    width: 100%;
    height: 100%;
    background: var(--panel-bg);
    display: flex;
    overflow: hidden;
    }

    .settings-sidebar {
    width: 250px;
    background: rgba(0, 0, 0, 0.2);
    border-right: 1px solid rgba(255, 255, 255, 0.05);
    padding: 32px;
    display: flex;
    flex-direction: column;
    }

    .settings-branding {
    margin-bottom: 48px;
    }

    .settings-nav {
    flex: 1;
    }

    .settings-tab {
    font-family: var(--font-display);
    font-size: 16px;
    text-transform: uppercase;
    letter-spacing: 1px;
    color: var(--muted);
    padding: 12px 0;
    cursor: pointer;
    transition: all 0.2s;
    }

    .settings-tab:hover { color: var(--text-color); }
    .settings-tab.active { color: var(--accent); }

    .settings-actions {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-top: auto;
    }

    .settings-save-btn {
    background: transparent;
    border: 1px solid var(--muted);
    color: var(--muted);
    padding: 10px;
    cursor: not-allowed;
    transition: all 0.2s;
    font-family: var(--font-display);
    text-transform: uppercase;
    letter-spacing: 1px;
    font-size: 12px;
    }

    .settings-save-btn.has-changes {
    border-color: var(--accent);
    color: var(--accent);
    cursor: pointer;
    }

    .settings-save-btn.has-changes:hover {
    background: var(--accent);
    color: #000;
    }

    .settings-discard-btn {
    background: transparent;
    border: 1px solid var(--muted);
    color: var(--muted);
    padding: 8px;
    cursor: not-allowed;
    transition: all 0.2s;
    font-family: var(--font-main);
    font-size: 11px;
    }

    .settings-discard-btn:not(:disabled) {
    cursor: pointer;
    }

    .settings-discard-btn:not(:disabled):hover {
    border-color: #c44;
    color: #c44;
    }

    .settings-back-btn {
    background: transparent;
    border: 1px solid var(--accent);
    color: var(--accent);
    padding: 10px;
    cursor: pointer;
    transition: all 0.2s;
    margin-top: 24px;
    }

    .settings-back-btn:hover {
    background: var(--accent);
    color: #000;
    }

    .settings-footer {
    margin-top: 24px;
    font-size: 11px;
    color: var(--muted);
    font-style: italic;
    }

    .settings-content {
    flex: 1;
    padding: 48px;
    overflow-y: auto;
    color: var(--text-color);
    }

    .settings-group {
    animation: fadeIn 0.3s ease-out;
    }

    .settings-field {
    margin-bottom: 24px;
    }

    .settings-label-row {
    margin-bottom: 8px;
    }

    .settings-select,.settings-input {
    display: block;
    width: 100%;
    background: #2a2a2a;
    border: 1px solid #444;
    color: var(--text-color);
    padding: 8px 12px;
    border-radius: 4px;
    font-family: var(--font-mono);
    font-size: 14px;
    }

    .settings-select option {
    background: #2a2a2a;
    color: var(--text-color);
    }

    .settings-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0 24px;
    }

    .settings-slider-container {
    display: flex;
    align-items: center;
    gap: 16px;
    }

    .settings-slider-container input {
    flex: 1;
    height: 4px;
    appearance: none;
    background: #444;
    border-radius: 2px;
    }

    .settings-slider-container input::-webkit-slider-thumb {
    appearance: none;
    width: 16px; height: 16px;
    background: var(--accent);
    border-radius: 50%;
    cursor: pointer;
    }

    .settings-toggle {
    display: flex;
    align-items: center;
    cursor: pointer;
    }

    .role-value { color: var(--accent); font-family: var(--font-mono); }

    @keyframes fadeIn {
    from { opacity: 0; transform: translateY(10px); }
    to { opacity: 1; transform: translateY(0); }
    }

    .settings-loading {
    font-family: var(--font-display);
    color: var(--accent);
    font-size: 24px;
    letter-spacing: 2px;
    }

    .v-list-editor {
    display: flex;
    flex-direction: column;
    gap: 12px;
    background: rgba(0, 0, 0, 0.1);
    padding: 12px;
    border: 1px solid rgba(255, 255, 255, 0.05);
    }
    .v-tags {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    }
    .v-tag {
    background: rgba(212, 175, 55, 0.1);
    border: 1px solid var(--accent);
    padding: 4px 10px;
    border-radius: 4px;
    display: flex;
    align-items: center;
    gap: 8px;
    animation: fadeIn 0.2s ease-out;
    }
    .v-tag span {
    font-size: 13px;
    color: var(--accent);
    font-family: var(--font-mono);
    }
    .v-tag button {
    background: transparent;
    border: none;
    color: var(--accent);
    cursor: pointer;
    font-size: 18px;
    padding: 0;
    line-height: 1;
    opacity: 0.6;
    transition: opacity 0.2s;
    }
    .v-tag button:hover { opacity: 1; }
    .v-input-row {
    display: flex;
    gap: 8px;
    }
    .v-presets {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 4px;
    }
    .v-preset-btn {
    background: transparent;
    border: 1px dashed rgba(255, 255, 255, 0.1);
    color: var(--muted);
    font-size: 11px;
    padding: 3px 8px;
    cursor: pointer;
    border-radius: 4px;
    transition: all 0.2s;
    }
    .v-preset-btn:hover {
    border-color: var(--accent);
    color: var(--accent);
    background: rgba(255, 255, 255, 0.02);
    }
`}</style>
        </div >
    );
};
