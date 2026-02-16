import React from 'react';

interface DebugBox {
    minX: number;
    minY: number;
    maxX: number;
    maxY: number;
    type: string;
    ownerId: string;
}

interface ArtisticMapDebugBoxesProps {
    boxes: DebugBox[];
}

export const ArtisticMapDebugBoxes: React.FC<ArtisticMapDebugBoxesProps> = ({ boxes }) => {
    return (
        <svg style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', pointerEvents: 'none', zIndex: 25 }}>
            {boxes.map((box, i) => (
                <rect
                    key={`dbg-${box.ownerId}-${i}`}
                    x={box.minX}
                    y={box.minY}
                    width={box.maxX - box.minX}
                    height={box.maxY - box.minY}
                    fill="none"
                    stroke={box.type === 'marker' ? 'rgba(255,60,60,0.7)' : 'rgba(60,120,255,0.7)'}
                    strokeWidth={1}
                />
            ))}
        </svg>
    );
};
