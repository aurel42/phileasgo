import React from 'react';
import { useTrackedPOIs } from '../hooks/usePOIs';
import type { Telemetry } from '../types/telemetry';
import { calculateDistance } from '../utils/geoUtils';

interface POIsCardProps {
    telemetry: Telemetry | null | undefined;
    onPlayPOI: (id: string, name: string) => void;
}

export const POIsCard: React.FC<POIsCardProps> = ({ telemetry, onPlayPOI }) => {
    const pois = useTrackedPOIs();

    const sortedPois = [...pois].map(p => {
        const distance = (telemetry && telemetry.Valid)
            ? calculateDistance(telemetry.Latitude, telemetry.Longitude, p.lat, p.lon)
            : 0;
        return { ...p, distance };
    }).sort((a, b) => a.distance - b.distance);

    return (
        <div className="flex-card" style={{ marginTop: '12px', padding: '12px 16px' }}>
            <div className="role-header" style={{ marginBottom: '8px' }}>
                Tracked Points of Interest
            </div>
            <div className="stats-container" style={{ maxHeight: '400px', overflowY: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                    <thead>
                        <tr style={{ textAlign: 'left' }}>
                            <th className="role-label" style={{ paddingBottom: '4px' }}>Name</th>
                            <th className="role-label" style={{ paddingBottom: '4px', textAlign: 'right' }}>Dist</th>
                            <th className="role-label" style={{ paddingBottom: '4px', textAlign: 'right' }}>Score</th>
                        </tr>
                    </thead>
                    <tbody>
                        {sortedPois.map((p) => (
                            <tr key={p.wikidata_id} style={{ borderTop: '1px solid rgba(255,255,255,0.05)' }}>
                                <td
                                    className="role-label clickable"
                                    style={{
                                        padding: '6px 0',
                                        color: 'var(--accent)',
                                        cursor: 'pointer',
                                        textDecoration: 'underline'
                                    }}
                                    onClick={() => onPlayPOI(p.wikidata_id, p.name_user || p.name_en || p.name_local)}
                                >
                                    {p.name_user || p.name_en || p.name_local}
                                    {p.is_on_cooldown && <span style={{ marginLeft: '8px', opacity: 0.5, fontSize: '0.8em' }}>(cooldown)</span>}
                                </td>
                                <td className="role-num-sm" style={{ padding: '6px 0', textAlign: 'right' }}>{p.distance.toFixed(1)}nm</td>
                                <td className="role-num-sm" style={{ padding: '6px 0', textAlign: 'right' }}>{p.score.toFixed(1)}</td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>
        </div>
    );
};
