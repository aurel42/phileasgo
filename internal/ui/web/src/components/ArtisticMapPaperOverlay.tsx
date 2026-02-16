import React from 'react';
import { ARTISTIC_MAP_STYLES } from '../styles/artisticMapStyles';
import { getMaskColor } from '../utils/artisticMapUtils';

interface ArtisticMapPaperOverlayProps {
    maskPath: string;
    simState: string | undefined;
    effectiveReplayMode: boolean;
    paperOpacityFog: number;
    paperOpacityClear: number;
    parchmentSaturation: number;
}

export const ArtisticMapPaperOverlay: React.FC<ArtisticMapPaperOverlayProps> = ({
    maskPath, simState, effectiveReplayMode, paperOpacityFog, paperOpacityClear, parchmentSaturation
}) => {
    const isIdleLocal = simState === 'disconnected' && !effectiveReplayMode;
    const useClearOpacity = effectiveReplayMode || isIdleLocal || simState === 'inactive';
    const baseOpacity = useClearOpacity ? paperOpacityClear : paperOpacityFog;

    return (
        <>
            {/* SVG Filter Definitions */}
            <svg style={{ position: 'absolute', width: 0, height: 0 }}>
                <defs>
                    <mask id="paper-mask" maskContentUnits="userSpaceOnUse">
                        <rect x="0" y="0" width="10000" height="10000" fill={getMaskColor(baseOpacity)} />
                        {!useClearOpacity && <path d={maskPath} fill={getMaskColor(paperOpacityClear)} />}
                    </mask>
                </defs>
            </svg>

            {/* Paper Overlay (Atomic from Frame) */}
            <div style={{
                position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', pointerEvents: 'none',
                backgroundColor: '#f4ecd8', backgroundImage: 'url(/assets/textures/paper.jpg), radial-gradient(#d4af37 1px, transparent 1px)',
                backgroundSize: 'cover, 20px 20px', zIndex: 10, mask: 'url(#paper-mask)', WebkitMask: 'url(#paper-mask)',
                filter: `saturate(${parchmentSaturation})`
            }} />
            <style>{`
    .stamped-icon { display: flex; justify-content: center; align-items: center; }
                .stamped-icon svg { width: 100%; height: 100%; overflow: visible; }
                .stamped-icon path, .stamped-icon circle, .stamped-icon rect, .stamped-icon polygon, .stamped-icon ellipse, .stamped-icon line {
    fill: currentColor!important;
    stroke: var(--stamped-stroke, ${ARTISTIC_MAP_STYLES.colors.icon.stroke}) !important;
    stroke-width: var(--stamped-width, 0.8px) !important;
    stroke-linejoin: round!important;
    vector-effect: non-scaling-stroke;
}
`}</style>
        </>
    );
};
