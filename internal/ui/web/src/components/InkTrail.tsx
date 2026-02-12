import React, { useMemo } from 'react';
import { interpolatePosition } from '../utils/replay';

interface InkTrailProps {
    pathPoints: [number, number][]; // [lat, lon][]
    progress: number;
    project: (lnglat: [number, number]) => { x: number, y: number };
}

export const InkTrail: React.FC<InkTrailProps> = ({ pathPoints, progress, project }) => {
    // 1. Interpolate to find where the "tip" is
    const { position, segmentIndex } = useMemo(() =>
        interpolatePosition(pathPoints, progress),
        [pathPoints, progress]);

    // 2. Project points to pixel space
    const projectedPoints = useMemo(() => {
        // Points up to the last full segment
        const segmentPoints = pathPoints.slice(0, segmentIndex + 1).map(p => {
            const pt = project([p[1], p[0]]);
            return `${pt.x},${pt.y}`;
        });

        // Add the interpolated tip
        const tipPt = project([position[1], position[0]]);
        segmentPoints.push(`${tipPt.x},${tipPt.y}`);

        return segmentPoints.join(' ');
    }, [pathPoints, segmentIndex, position, project]);

    if (pathPoints.length < 2) return null;

    return (
        <svg
            style={{
                position: 'absolute',
                left: 0,
                top: 0,
                width: '100%',
                height: '100%',
                pointerEvents: 'none',
                zIndex: 40, // Below balloon, above terrain
                overflow: 'visible'
            }}
        >
            <defs>
                <filter id="ink-bleed-route" x="-20%" y="-20%" width="140%" height="140%">
                    <feTurbulence type="fractalNoise" baseFrequency="0.04" numOctaves="2" result="noise" />
                    <feDisplacementMap in="SourceGraphic" in2="noise" scale="1.5" />
                </filter>
            </defs>

            <g filter="url(#ink-bleed-route)">
                {/* Bleeding Shadow/Bleed Effect (Dark Red/Brown) */}
                <polyline
                    points={projectedPoints}
                    fill="none"
                    stroke="#8b1a1a" // Deep Blood Red
                    strokeWidth="4"
                    strokeOpacity="0.4"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeDasharray="8,4" // Matching the Victorian travel aesthetic
                />

                {/* Main Ink Core (Crimson) */}
                <polyline
                    points={projectedPoints}
                    fill="none"
                    stroke="#e63946" // Vibrant Crimson
                    strokeWidth="1.8"
                    strokeOpacity="0.9"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeDasharray="8,4"
                />
            </g>
        </svg>
    );
};
