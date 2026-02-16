import React from 'react';

interface POIBeaconProps {
    color: string;
    size: number;
    showHalo?: boolean;
    activeBoost?: number;
    zoomScale?: number;
    x: number;
    y: number;
    iconSize?: number; // Size of the icon below the beacon
}

export const POIBeacon: React.FC<POIBeaconProps> = ({ color, size, showHalo = true, zoomScale = 1, x, y, iconSize = 32 }) => {
    return (
        <svg
            viewBox="0 0 40 50"
            style={{
                position: 'absolute',
                left: x,
                top: y,
                width: size,
                height: size * 1.25,
                // Dynamic offset calculation:
                // Displacement consists of: (iconSize / 2) + 2px (gap) + 7.5px (balloon radius)
                // Using transform ensures the displacement scales perfectly along with the balloon and icon.
                transform: `translate(-50%, -50%) scale(${zoomScale}) translateY(-${(iconSize / 2) + 2 + 7.5}px)`,
                zIndex: 110,
                filter: showHalo ? 'drop-shadow(0 0 1px white)' : 'none',
                pointerEvents: 'none'
            }}
        >
            {/* Envelope */}
            <path
                d="M20,5 C12,5 5,12 5,22 C5,28 10,35 20,42 C30,35 35,28 35,22 C35,12 28,5 20,5"
                fill={color}
                stroke="black"
                strokeWidth="1.5"
            />
            {/* Strings */}
            <line x1="12" y1="36" x2="16" y2="42" stroke="black" strokeWidth="1" />
            <line x1="28" y1="36" x2="24" y2="42" stroke="black" strokeWidth="1" />
            {/* Gondola/Basket */}
            <rect x="16" y="42" width="8" height="6" rx="1" fill="#1a1a1a" stroke="black" strokeWidth="1" />
        </svg>
    );
};
