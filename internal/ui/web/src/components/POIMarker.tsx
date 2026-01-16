
import React, { useMemo } from 'react';
import { Marker } from 'react-leaflet';
import L from 'leaflet';
import type { POI } from '../hooks/usePOIs';


interface POIMarkerProps {
    poi: POI;
    highlighted?: boolean;
    preparing?: boolean;
    onClick: (poi: POI) => void;
}

const getColor = (score: number) => {
    // Score 1 (Yellow, Hue 60) -> Score 50 (Red, Hue 0)
    const clamped = Math.max(1, Math.min(50, score));
    const ratio = (clamped - 1) / 49;
    const hue = 60 - (ratio * 60);
    return `hsl(${hue}, 100%, 50%)`;
};

export const POIMarker = React.memo(({ poi, highlighted, preparing, onClick }: POIMarkerProps) => {
    // Memoize icon to prevent flickering/re-creation on every render
    const icon = useMemo(() => {
        // Safe check for icon, default if missing
        const iconName = poi.icon && poi.icon.length > 0 ? poi.icon : 'attraction';
        const iconPath = `/icons/${iconName}.svg`;

        const isPlayed = poi.last_played && poi.last_played !== "0001-01-01T00:00:00Z";

        // Colors: Green for playing, Dark Green for preparing, Blue for played, Score-based for others
        let bgColor = getColor(poi.score);
        let scale = 1.0;

        if (highlighted) {
            bgColor = '#22c55e'; // Vibrant Green (playing/selected)
            scale = 1.5;
        } else if (preparing) {
            bgColor = '#166534'; // Dark Green (preparing)
            scale = 1.25;
        } else if (isPlayed) {
            bgColor = '#3b82f6'; // Vibrant Blue
        }

        const borderWidth = 2;
        const borderColor = bgColor;
        const shadow = '0 2px 4px rgba(0, 0, 0, 0.5)';

        const starBadge = poi.is_msfs_poi ? `<div style="
            position: absolute;
            top: -6px;
            right: -6px;
            color: #fbbf24;
            filter: drop-shadow(0 1px 1px rgba(0,0,0,0.5));
            z-index: 10;
            font-size: 16px;
            line-height: 1;
        ">★</div>` : '';

        return L.divIcon({
            className: `poi-marker-container ${highlighted ? 'highlighted' : ''} ${preparing ? 'preparing' : ''}`,
            html: `<div class="poi-marker-bg" style="
                position: relative;
                background-color: ${bgColor}; 
                border: ${borderWidth}px solid ${borderColor}; 
                width: 32px; height: 32px; 
                transform: scale(${scale}); 
                box-shadow: ${shadow};
                transition: all 0.3s ease;
            ">
                <img src="${iconPath}" style="width: 24px; height: 24px;" />
                ${starBadge}
            </div>`,
            iconSize: [32, 32],
            iconAnchor: [16, 16],
        });
    }, [poi.icon, poi.score, poi.last_played, poi.is_msfs_poi, highlighted, preparing]);

    return (
        <Marker
            position={[poi.lat, poi.lon]}
            icon={icon}
            zIndexOffset={highlighted ? 2000 : (poi.is_msfs_poi ? 1000 : 0)}
            eventHandlers={{
                click: () => onClick(poi)
            }}
        />
    );
});

