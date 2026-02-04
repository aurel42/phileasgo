import { useEffect, useState, useRef } from 'react';
import type { Telemetry } from '../types/telemetry';

interface OverlayTelemetryBarProps {
    telemetry?: Telemetry;
}

interface Stats {
    providers?: {
        wikidata?: { api_success: number; api_zero: number; api_errors: number; hit_rate: number; free_tier?: boolean };
        wikipedia?: { api_success: number; api_errors: number; hit_rate: number; free_tier?: boolean };
        gemini?: { api_success: number; api_errors: number; free_tier?: boolean };
        'edge-tts'?: { api_success: number; api_zero: number; api_errors: number; free_tier?: boolean };
        'azure-speech'?: { api_success: number; api_zero: number; api_errors: number; free_tier?: boolean };
        [key: string]: { api_success: number; api_zero?: number; api_errors: number; hit_rate?: number; free_tier?: boolean } | undefined;
    };
    system?: { memory_alloc_mb: number; memory_max_mb: number; goroutines: number };
    tracking?: { active_pois: number };
}

interface Config {
    filter_mode?: string;
    target_poi_count?: number;
    min_poi_score?: number;
    narration_frequency?: number;
    text_length?: number;
    llm_provider?: string;
    tts_engine?: string;
    show_log_line?: boolean;
}

import { useGeography } from '../hooks/useGeography';
import { TelemetryBox } from './overlay/TelemetryBox';
import { PositionBox } from './overlay/PositionBox';
import { PipelineBox } from './overlay/PipelineBox';
import { DataServicesBox } from './overlay/DataServicesBox';
import { StatsBox } from './overlay/StatsBox';
import { BrandingBox } from './overlay/BrandingBox';
import { ConfigBox } from './overlay/ConfigBox';

export const OverlayTelemetryBar = ({ telemetry }: OverlayTelemetryBarProps) => {
    const [stats, setStats] = useState<Stats | null>(null);
    const [version, setVersion] = useState<string>('...');
    const [config, setConfig] = useState<Config>({});
    const { location } = useGeography(telemetry);
    const [logLine, setLogLine] = useState<string>('');

    // Use ref to access latest telemetry in interval without resetting it
    const telemetryRef = useRef(telemetry);
    useEffect(() => { telemetryRef.current = telemetry; }, [telemetry]);

    useEffect(() => {
        const fetchStats = () => {
            fetch('/api/stats')
                .then(r => r.json())
                .then(data => setStats(data))
                .catch(() => { });
        };

        const fetchVersion = () => {
            fetch('/api/version')
                .then(r => r.json())
                .then(data => setVersion(data.version || '?'))
                .catch(() => { });
        };

        const fetchConfig = () => {
            fetch('/api/config')
                .then(r => r.json())
                .then(data => setConfig(data))
                .catch(() => { });
        };

        const fetchLog = () => {
            fetch('/api/log/latest')
                .then(r => r.json())
                .then(data => setLogLine(data.log || ''))
                .catch(() => { });
        };

        fetchStats();
        fetchVersion();
        fetchConfig();
        fetchLog();

        const statsInterval = setInterval(fetchStats, 5000);
        const configInterval = setInterval(fetchConfig, 5000);
        const logInterval = setInterval(fetchLog, 1000);

        return () => {
            clearInterval(statsInterval);
            clearInterval(configInterval);
            clearInterval(logInterval);
        };
    }, []);

    if (!telemetry || telemetry.SimState === 'disconnected' || !telemetry.Valid) {
        return (
            <div className="overlay-telemetry-bar">
                <div className="stats-row">
                    <div className="stat-box">
                        <div className="stat-value">
                            <span className="status-dot error"></span>
                            Disconnected
                        </div>
                    </div>
                </div>
            </div>
        );
    }

    return (
        <div className="overlay-telemetry-bar">
            <div className="stats-row">
                <TelemetryBox telemetry={telemetry} />
                <PositionBox telemetry={telemetry} location={location} />
                <PipelineBox stats={stats} />
                <DataServicesBox stats={stats} />
                <StatsBox stats={stats} />
                <BrandingBox version={version} />
                <ConfigBox telemetry={telemetry} config={config} />
            </div>

            {config.show_log_line && (
                <div className="log-line role-label-overlay" style={{ fontStyle: 'italic', fontSize: '16px', lineHeight: '30px' }}>
                    {logLine}
                </div>
            )}
        </div>
    );
};
