import React from 'react';
import type { CreditItem } from '../utils/replay';

interface CreditRollProps {
    items: CreditItem[];
    totalPOICount: number;
    mapContainer: HTMLElement | null;
    currentTime: number;
}

export const CreditRoll: React.FC<CreditRollProps> = ({ items, totalPOICount, mapContainer, currentTime }) => {
    const now = currentTime;

    // Adaptive timing: more POIs = faster scroll
    const visibleDuration = Math.max(3000, 9000 - (totalPOICount * 75));

    // Filter to items still in view (not yet scrolled off)
    const visibleItems = items.filter(item => {
        const age = now - item.addedAt;
        return age < visibleDuration;
    });

    if (visibleItems.length === 0 || !mapContainer) return null;

    // Get map bounds for positioning
    const mapRect = mapContainer.getBoundingClientRect();
    const mapHeight = mapRect.height;

    return (
        <div style={{
            position: 'fixed',
            top: mapRect.top,
            left: mapRect.left,
            width: mapRect.width,
            height: mapRect.height,
            overflow: 'hidden', // Clip items outside map bounds
            pointerEvents: 'none',
            zIndex: 9999,
        }}>
            {visibleItems.map((item) => {
                const age = now - item.addedAt;
                const progress = age / visibleDuration;

                // Scroll from bottom to top of map
                const yPos = mapHeight * (1 - progress);

                return (
                    <div
                        key={item.id}
                        className="role-title"
                        style={{
                            position: 'absolute',
                            left: '50%',
                            top: yPos,
                            transform: 'translate(-50%, -50%)',
                            fontSize: '18px',
                            fontWeight: 500,
                            // White text with black outline
                            color: '#ffffff',
                            textShadow: `
                                -1px -1px 0 #000,
                                1px -1px 0 #000,
                                -1px 1px 0 #000,
                                1px 1px 0 #000,
                                0 0 4px rgba(0,0,0,0.8)
                            `,
                            textAlign: 'center',
                            maxWidth: '90%',
                            padding: '0 5%',
                        }}
                    >
                        {item.name}
                    </div>
                );
            })}
        </div>
    );
};
