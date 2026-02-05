import React, { useState, useEffect, useCallback } from 'react';
import { VictorianToggle } from './VictorianToggle';
import type { Telemetry } from '../types/telemetry';
import { DualRangeSlider } from './DualRangeSlider';

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
    streamingMode: boolean;
    onStreamingModeChange: (val: boolean) => void;
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
    mockDurationParked: string;
    mockDurationTaxi: string;
    mockDurationHold: string;
    // Interface tab (local-only, no server sync needed)
    units: 'km' | 'nm';
    showCacheLayer: boolean;
    showVisibilityLayer: boolean;
    streamingMode: boolean;
    // Scorer tab
    deferralProximityBoostPower: number;
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
    streamingMode,
    onStreamingModeChange
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
                    narrationFrequency,
                    textLength,
                    promptUnits: data.units || 'hybrid',
                    minPoiScore,
                    filterMode,
                    targetPoiCount,
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
                    mockDurationParked: data.mock_duration_parked || '',
                    mockDurationTaxi: data.mock_duration_taxi || '',
                    mockDurationHold: data.mock_duration_hold || '',
                    units,
                    showCacheLayer,
                    showVisibilityLayer,
                    streamingMode,
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
                });
                setLoading(false);
            })
            .catch(e => console.error("Failed to fetch settings", e));
    }, []);

    // Check if draft differs from original
    const hasChanges = useCallback(() => {
        if (!draft || !serverConfig) return false;
        return (
            draft.narrationFrequency !== narrationFrequency ||
            draft.textLength !== textLength ||
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
            draft.mockDurationParked !== (serverConfig.mock_duration_parked || '') ||
            draft.mockDurationTaxi !== (serverConfig.mock_duration_taxi || '') ||
            draft.mockDurationHold !== (serverConfig.mock_duration_hold || '') ||
            draft.units !== units ||
            draft.showCacheLayer !== showCacheLayer ||
            draft.showVisibilityLayer !== showVisibilityLayer ||
            draft.streamingMode !== streamingMode ||
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
            draft.beaconMaxTargets !== (serverConfig.beacon_max_targets ?? 2)
        );
    }, [draft, serverConfig, narrationFrequency, textLength, minPoiScore, filterMode, targetPoiCount, units, showCacheLayer, showVisibilityLayer, streamingMode]);

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
        if (draft.mockDurationParked !== (serverConfig?.mock_duration_parked || '')) payload.mock_duration_parked = draft.mockDurationParked;
        if (draft.mockDurationTaxi !== (serverConfig?.mock_duration_taxi || '')) payload.mock_duration_taxi = draft.mockDurationTaxi;
        if (draft.mockDurationHold !== (serverConfig?.mock_duration_hold || '')) payload.mock_duration_hold = draft.mockDurationHold;
        if (draft.deferralProximityBoostPower !== (serverConfig?.deferral_proximity_boost_power ?? 1.0)) payload.deferral_proximity_boost_power = draft.deferralProximityBoostPower;
        if (draft.twoPassScriptGeneration !== (serverConfig?.two_pass_script_generation ?? false)) payload.two_pass_script_generation = draft.twoPassScriptGeneration;
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
            if (draft.streamingMode !== streamingMode) onStreamingModeChange(draft.streamingMode);

            // Apply prop-based changes via callbacks (these update parent state AND send to server)
            if (draft.narrationFrequency !== narrationFrequency) onNarrationFrequencyChange(draft.narrationFrequency);
            if (draft.textLength !== textLength) onTextLengthChange(draft.textLength);
            if (draft.minPoiScore !== minPoiScore) onMinPoiScoreChange(draft.minPoiScore);
            if (draft.filterMode !== filterMode) onFilterModeChange(draft.filterMode);
            if (draft.targetPoiCount !== targetPoiCount) onTargetPoiCountChange(draft.targetPoiCount);

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
        { id: 'interface', label: 'Interface' }
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
                                                value={draft.mockDurationParked}
                                                onChange={e => updateDraft('mockDurationParked', e.target.value)}
                                            />
                                        ))}
                                        {renderField('Taxi Duration', (
                                            <input
                                                type="text"
                                                className="settings-input"
                                                placeholder="e.g. 2m"
                                                value={draft.mockDurationTaxi}
                                                onChange={e => updateDraft('mockDurationTaxi', e.target.value)}
                                            />
                                        ))}
                                        {renderField('Hold Duration', (
                                            <input
                                                type="text"
                                                className="settings-input"
                                                placeholder="e.g. 10s"
                                                value={draft.mockDurationHold}
                                                onChange={e => updateDraft('mockDurationHold', e.target.value)}
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
                            <VictorianToggle
                                label="2-pass script generation"
                                checked={draft.twoPassScriptGeneration}
                                onChange={val => updateDraft('twoPassScriptGeneration', val)}
                            />

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

                    {activeTab === 'interface' && (
                        <div className="settings-group">
                            <div className="role-header">Units & Display</div>
                            {/* DO NOT CHANGE: This specifically controls the range ring spacing and units on the map */}
                            {renderField('Range Ring Units', (
                                <select className="settings-select" value={draft.units} onChange={e => updateDraft('units', e.target.value as 'km' | 'nm')}>
                                    <option value="km">km</option>
                                    <option value="nm">nm</option>
                                </select>
                            ))}

                            <div className="role-header" style={{ marginTop: '24px' }}>Overlay Layers</div>
                            <VictorianToggle
                                label="Show POI Cache Radius"
                                checked={draft.showCacheLayer}
                                onChange={val => updateDraft('showCacheLayer', val)}
                            />
                            <VictorianToggle
                                label="Show Line-of-Sight Coverage"
                                checked={draft.showVisibilityLayer}
                                onChange={val => updateDraft('showVisibilityLayer', val)}
                            />

                            <div className="role-header" style={{ marginTop: '24px' }}>Developer Settings</div>
                            <VictorianToggle
                                label="Streaming Mode (LocalStorage)"
                                checked={draft.streamingMode}
                                onChange={val => updateDraft('streamingMode', val)}
                            />
                        </div>
                    )}
                    {activeTab === 'scorer' && (
                        <div className="settings-group">
                            <div className="role-header">Visibility & Deferral</div>
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
                        </div>
                    )}
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
                                        min="500" max="10000" step="500"
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
        </div>
    );
};
