import React from 'react';
import { useSettlements } from '../hooks/useSettlements';
import type { Telemetry } from '../types/telemetry';
import { calculateDistance } from '../utils/geoUtils';

interface CitiesCardProps {
    telemetry: Telemetry | null | undefined;
    onPlayCity: (id: string, name: string) => void;
}

export const CitiesCard: React.FC<CitiesCardProps> = ({ telemetry, onPlayCity }) => {
    const settlements = useSettlements(telemetry);

    const sortedSettlements = [...settlements].map(s => {
        const distance = (telemetry && telemetry.Valid)
            ? calculateDistance(telemetry.Latitude, telemetry.Longitude, s.lat, s.lon)
            : 0;
        return { ...s, distance };
    }).sort((a, b) => {
        if (Math.abs(a.distance - b.distance) < 0.01) {
            return a.name.localeCompare(b.name);
        }
        return a.distance - b.distance;
    });

    return (
        <div className="flex-card" style={{ marginTop: '12px', padding: '12px 16px' }}>
            <div className="role-header" style={{ marginBottom: '8px' }}>
                Nearby Cities & Settlements
            </div>
            <div className="stats-container" style={{ maxHeight: '400px', overflowY: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                    <thead>
                        <tr style={{ textAlign: 'left' }}>
                            <th className="role-label" style={{ paddingBottom: '4px' }}>Name</th>
                            <th className="role-label" style={{ paddingBottom: '4px', textAlign: 'right' }}>Dist</th>
                            <th className="role-label" style={{ paddingBottom: '4px', textAlign: 'right' }}>Pop</th>
                            <th className="role-label" style={{ paddingBottom: '4px', textAlign: 'right' }}>Cat</th>
                        </tr>
                    </thead>
                    <tbody>
                        {sortedSettlements.map((s) => (
                            <tr key={s.id} style={{ borderTop: '1px solid rgba(255,255,255,0.05)' }}>
                                <td
                                    className="role-label clickable"
                                    style={{
                                        padding: '6px 0',
                                        color: 'var(--accent)',
                                        cursor: 'pointer',
                                        textDecoration: 'underline'
                                    }}
                                    onClick={() => onPlayCity(s.id, s.name)}
                                >
                                    {s.name}
                                </td>
                                <td className="role-num-sm" style={{ padding: '6px 0', textAlign: 'right' }}>{s.distance.toFixed(1)}nm</td>
                                <td className="role-num-sm" style={{ padding: '6px 0', textAlign: 'right' }}>{s.pop.toLocaleString()}</td>
                                <td className="role-label-sm" style={{ padding: '6px 0', textAlign: 'right', opacity: 0.7 }}>{s.category}</td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>
        </div>
    );
};
