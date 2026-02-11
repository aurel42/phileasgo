import React from 'react';

export const CompassRose: React.FC<{ size: number }> = ({ size }) => {
    return (
        <svg
            viewBox="0 0 100 100"
            width={size}
            height={size}
            style={{ overflow: 'visible' }}
        >
            <defs>
                <filter id="ink-bleed-compass-v5" x="-20%" y="-20%" width="140%" height="140%">
                    <feTurbulence type="fractalNoise" baseFrequency="0.05" numOctaves="2" result="noise" />
                    <feDisplacementMap in="SourceGraphic" in2="noise" scale="0.8" />
                </filter>
            </defs>
            <g filter="url(#ink-bleed-compass-v5)">
                {/* Through Lines (Opposite Points) */}
                <line x1="50" y1="5" x2="50" y2="95" stroke="currentColor" strokeWidth="0.3" opacity="0.4" />
                <line x1="5" y1="50" x2="95" y2="50" stroke="currentColor" strokeWidth="0.3" opacity="0.4" />

                {/* Sub-Star (Diagonals) - Slimmer, solid, fully dark */}
                <g opacity="0.8">
                    {/* NE */}
                    <path d="M50,50 L71,29 L62,40 Z" fill="currentColor" />
                    <path d="M50,50 L71,29 L40,62 Z" fill="currentColor" opacity="0.1" /> {/* Subtle balancing stroke */}
                    {/* SE */}
                    <path d="M50,50 L71,71 L62,60 Z" fill="currentColor" />
                    {/* SW */}
                    <path d="M50,50 L29,71 L40,60 Z" fill="currentColor" />
                    {/* NW */}
                    <path d="M50,50 L29,29 L40,40 Z" fill="currentColor" />
                </g>

                {/* Circle Intersecting at 2/3rds (Radius 30) */}
                <circle cx="50" cy="50" r="30" fill="none" stroke="currentColor" strokeWidth="0.5" opacity="0.6" />

                {/* Main Star (Cardinals) - 3D look */}
                {/* North */}
                <path d="M50,5 L54,45 L50,50 Z" fill="currentColor" />
                <path d="M50,5 L46,45 L50,50 Z" fill="none" stroke="currentColor" strokeWidth="0.3" />
                {/* South */}
                <path d="M50,95 L46,55 L50,50 Z" fill="currentColor" />
                <path d="M50,95 L54,55 L50,50 Z" fill="none" stroke="currentColor" strokeWidth="0.3" />
                {/* East */}
                <path d="M95,50 L55,54 L50,50 Z" fill="currentColor" />
                <path d="M95,50 L55,46 L50,50 Z" fill="none" stroke="currentColor" strokeWidth="0.3" />
                {/* West */}
                <path d="M5,50 L45,46 L50,50 Z" fill="currentColor" />
                <path d="M5,50 L45,54 L50,50 Z" fill="none" stroke="currentColor" strokeWidth="0.3" />

                {/* Cardinal Label - Only North, prominent */}
                <text x="50" y="-8" textAnchor="middle" fontSize="22" fontFamily="serif" fill="currentColor" fontWeight="bold">N</text>

                {/* Center Hub */}
                <circle cx="50" cy="50" r="1.8" fill="none" stroke="currentColor" strokeWidth="0.4" />
                <circle cx="50" cy="50" r="0.7" fill="currentColor" />
            </g>
        </svg>
    );
};
