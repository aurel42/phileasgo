import React from 'react';
import type { LabelCandidate } from '../metrics/PlacementEngine';
import { ARTISTIC_MAP_STYLES } from '../styles/artisticMapStyles';

interface ArtisticMapSettlementProps {
    label: LabelCandidate;
    finalX: number;
    finalY: number;
    zoomScale: number;
    fadeOpacity: number;
}

export const ArtisticMapSettlement: React.FC<ArtisticMapSettlementProps> = ({ label: l, finalX, finalY, zoomScale, fadeOpacity }) => {
    return (
        <React.Fragment key={l.id}>
            <div
                className={l.tier === 'city' ? 'role-title' : (l.tier === 'town' ? 'role-header' : 'role-text-lg')}
                style={{
                    position: 'absolute', left: finalX, top: finalY, transform: `translate(-50%, -50%) scale(${zoomScale})`,
                    fontSize: l.tier === 'city' ? '24px' : (l.tier === 'town' ? '16px' : '14px'), // Match role font adjustments (-4)
                    color: l.isHistorical ? ARTISTIC_MAP_STYLES.colors.text.historical : ARTISTIC_MAP_STYLES.colors.text.active,
                    textShadow: ARTISTIC_MAP_STYLES.colors.shadows.atmosphere,
                    whiteSpace: 'nowrap',
                    pointerEvents: 'none',
                    zIndex: 5,
                    opacity: fadeOpacity
                }}
            >
                {l.text}
            </div>
        </React.Fragment>
    );
};
