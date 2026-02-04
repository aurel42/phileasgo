import React, { useState } from 'react';
import type { Telemetry } from '../types/telemetry';

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
    const [activeTab, setActiveTab] = useState('sim');
    const [config, setConfig] = useState<any>(null);
    const [loading, setLoading] = useState(true);

    React.useEffect(() => {
        fetch('/api/config')
            .then(r => r.json())
            .then(data => {
                setConfig(data);
                setLoading(false);
            })
            .catch(e => console.error("Failed to fetch settings", e));
    }, []);

    const updateValue = (key: string, val: any) => {
        // Optimistic local state update for Mock fields
        if (key.startsWith('mock_') || key === 'teleport_distance') {
            setConfig((prev: any) => ({ ...prev, [key]: val }));
        }

        fetch('/api/config', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ [key]: val })
        }).catch(e => console.error("Failed to update config", e));
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
        { id: 'sim', label: 'Simulator' },
        { id: 'narrator', label: 'Narrator' },
        { id: 'interface', label: 'Interface' }
    ];

    if (loading) {
        return (
            <div className="settings-overlay">
                <div className="settings-loading">Consulting the Archives...</div>
            </div>
        );
    }

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
                    <button className="settings-back-btn role-btn" onClick={onBack}>
                        Return to Map
                    </button>
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
                                    value={config?.sim_source || 'simconnect'}
                                    onChange={e => updateValue('sim_source', e.target.value)}
                                >
                                    <option value="simconnect">Microsoft Flight Simulator</option>
                                    <option value="mock">Mock Simulator (Internal)</option>
                                </select>
                            ), true)}

                            {config?.sim_source === 'mock' && (
                                <>
                                    <div className="role-header" style={{ marginTop: '24px' }}>Mock Simulator Parameters</div>
                                    <div className="settings-grid">
                                        {renderField('Start Latitude', (
                                            <input
                                                type="number"
                                                className="settings-input"
                                                value={config?.mock_start_lat ?? ''}
                                                onChange={e => updateValue('mock_start_lat', parseFloat(e.target.value))}
                                            />
                                        ))}
                                        {renderField('Start Longitude', (
                                            <input
                                                type="number"
                                                className="settings-input"
                                                value={config?.mock_start_lon ?? ''}
                                                onChange={e => updateValue('mock_start_lon', parseFloat(e.target.value))}
                                            />
                                        ))}
                                        {renderField('Start Altitude (ft)', (
                                            <input
                                                type="number"
                                                className="settings-input"
                                                value={config?.mock_start_alt ?? ''}
                                                onChange={e => updateValue('mock_start_alt', parseFloat(e.target.value))}
                                            />
                                        ))}
                                        {renderField('Start Heading (deg)', (
                                            <input
                                                type="number"
                                                className="settings-input"
                                                placeholder="Automatic"
                                                value={config?.mock_start_heading ?? ''}
                                                onChange={e => updateValue('mock_start_heading', e.target.value === '' ? null : parseFloat(e.target.value))}
                                            />
                                        ))}
                                    </div>
                                    <div className="settings-grid">
                                        {renderField('Parked Duration', (
                                            <input
                                                type="text"
                                                className="settings-input"
                                                placeholder="e.g. 30s, 1m"
                                                value={config?.mock_duration_parked ?? ''}
                                                onChange={e => updateValue('mock_duration_parked', e.target.value)}
                                            />
                                        ))}
                                        {renderField('Taxi Duration', (
                                            <input
                                                type="text"
                                                className="settings-input"
                                                placeholder="e.g. 2m"
                                                value={config?.mock_duration_taxi ?? ''}
                                                onChange={e => updateValue('mock_duration_taxi', e.target.value)}
                                            />
                                        ))}
                                        {renderField('Hold Duration', (
                                            <input
                                                type="text"
                                                className="settings-input"
                                                placeholder="e.g. 10s"
                                                value={config?.mock_duration_hold ?? ''}
                                                onChange={e => updateValue('mock_duration_hold', e.target.value)}
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
                                    <span className="role-value">{['Rare', 'Occasional', 'Normal', 'Frequent', 'Constant'][narrationFrequency - 1] || narrationFrequency}</span>
                                    <input type="range" min="1" max="5" value={narrationFrequency} onChange={e => onNarrationFrequencyChange(parseInt(e.target.value))} />
                                </div>
                            ))}
                            {renderField('Script Length', (
                                <div className="settings-slider-container">
                                    <span className="role-value">{['Short', 'Brief', 'Normal', 'Detailed', 'Long'][textLength - 1] || textLength}</span>
                                    <input type="range" min="1" max="5" value={textLength} onChange={e => onTextLengthChange(parseInt(e.target.value))} />
                                </div>
                            ))}

                            <div className="role-header" style={{ marginTop: '24px' }}>POI Selection</div>
                            {renderField('Minimum POI Score', (
                                <div className="settings-slider-container">
                                    <span className="role-value">{Math.round(minPoiScore * 100)}%</span>
                                    <input type="range" min="0" max="1" step="0.05" value={minPoiScore} onChange={e => onMinPoiScoreChange(parseFloat(e.target.value))} />
                                </div>
                            ))}
                            {renderField('POI Filtering Mode', (
                                <select className="settings-select" value={filterMode} onChange={e => onFilterModeChange(e.target.value)}>
                                    <option value="fixed">Fixed Count</option>
                                    <option value="adaptive">Adaptive Radius</option>
                                </select>
                            ))}
                            {filterMode === 'adaptive' ?
                                renderField('Target POI Count', (
                                    <div className="settings-slider-container">
                                        <span className="role-value">{targetPoiCount}</span>
                                        <input type="range" min="1" max="50" value={targetPoiCount} onChange={e => onTargetPoiCountChange(parseInt(e.target.value))} />
                                    </div>
                                )) :
                                renderField('Teleport Threshold', (
                                    <div className="settings-slider-container">
                                        <span className="role-value">{config?.teleport_distance ?? 10} km</span>
                                        <input
                                            type="range"
                                            min="1" max="100"
                                            value={config?.teleport_distance ?? 10}
                                            onChange={e => updateValue('teleport_distance', parseInt(e.target.value))}
                                        />
                                    </div>
                                ))
                            }
                        </div>
                    )}

                    {activeTab === 'interface' && (
                        <div className="settings-group">
                            <div className="role-header">Units & Display</div>
                            {renderField('Measurement Units', (
                                <select className="settings-select" value={units} onChange={e => onUnitsChange(e.target.value as 'km' | 'nm')}>
                                    <option value="km">Metric (km/m)</option>
                                    <option value="nm">Nautical (nm/ft)</option>
                                </select>
                            ))}

                            <div className="role-header" style={{ marginTop: '24px' }}>Overlay Layers</div>
                            {renderField('Show POI Cache Radius', (
                                <label className="settings-toggle">
                                    <input type="checkbox" checked={showCacheLayer} onChange={e => onCacheLayerChange(e.target.checked)} />
                                    <span className="toggle-slider"></span>
                                    <span className="role-value" style={{ marginLeft: '12px' }}>{showCacheLayer ? 'ENABLED' : 'DISABLED'}</span>
                                </label>
                            ))}
                            {renderField('Show Line-of-Sight Coverage', (
                                <label className="settings-toggle">
                                    <input type="checkbox" checked={showVisibilityLayer} onChange={e => onVisibilityLayerChange(e.target.checked)} />
                                    <span className="toggle-slider"></span>
                                    <span className="role-value" style={{ marginLeft: '12px' }}>{showVisibilityLayer ? 'ENABLED' : 'DISABLED'}</span>
                                </label>
                            ))}

                            <div className="role-header" style={{ marginTop: '24px' }}>Developer Settings</div>
                            {renderField('Streaming Mode (LocalStorage)', (
                                <label className="settings-toggle">
                                    <input type="checkbox" checked={streamingMode} onChange={e => onStreamingModeChange(e.target.checked)} />
                                    <span className="toggle-slider"></span>
                                    <span className="role-value" style={{ marginLeft: '12px' }}>{streamingMode ? 'ENABLED' : 'DISABLED'}</span>
                                </label>
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
                    justify-content: center;
                    align-items: center;
                    font-family: var(--font-main);
                }

                .settings-container {
                    width: 900px;
                    height: 650px;
                    background: var(--panel-bg);
                    border: 3px double rgba(212, 175, 55, 0.3);
                    box-shadow: 0 20px 50px rgba(0,0,0,0.8);
                    display: flex;
                    overflow: hidden;
                }

                .settings-sidebar {
                    width: 250px;
                    background: rgba(0,0,0,0.2);
                    border-right: 1px solid rgba(255,255,255,0.05);
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

                .settings-select, .settings-input {
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

                .settings-toggle input { display: none; }
                .toggle-slider {
                    width: 40px; height: 20px;
                    background: #444;
                    border-radius: 10px;
                    position: relative;
                    transition: 0.3s;
                }

                .toggle-slider::after {
                    content: '';
                    position: absolute;
                    width: 14px; height: 14px;
                    background: #888;
                    border-radius: 50%;
                    top: 3px; left: 3px;
                    transition: 0.3s;
                }

                input:checked + .toggle-slider { background: var(--accent); }
                input:checked + .toggle-slider::after { left: 23px; background: #000; }

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
            `}</style>
        </div>
    );
};
