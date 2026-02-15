import React from 'react';
import type { LabelCandidate } from '../../metrics/PlacementEngine';
import { ARTISTIC_MAP_STYLES } from '../../styles/artisticMapStyles';

interface SettlementMarkerProps {
    label: LabelCandidate;
    finalX: number;
    finalY: number;
    zoomScale: number;
    fadeOpacity: number;
}

export const SettlementMarker: React.FC<SettlementMarkerProps> = React.memo(({
    label, finalX, finalY, zoomScale, fadeOpacity
}) => {
    return (
        <div
            className={label.tier === 'city' ? 'role-title' : (label.tier === 'town' ? 'role-header' : 'role-text-lg')}
            style={{
                position: 'absolute', left: finalX, top: finalY, transform: `translate(-50%, -50%) scale(${zoomScale})`,
                fontSize: label.tier === 'city' ? '24px' : (label.tier === 'town' ? '16px' : '14px'),
                color: label.isHistorical ? ARTISTIC_MAP_STYLES.colors.text.historical : ARTISTIC_MAP_STYLES.colors.text.active,
                textShadow: ARTISTIC_MAP_STYLES.colors.shadows.atmosphere,
                whiteSpace: 'nowrap',
                pointerEvents: 'none',
                zIndex: 5,
                opacity: fadeOpacity
            }}
        >
            {label.text}
        </div>
    );
});
