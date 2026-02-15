import React from 'react';
import type { Telemetry } from '../../types/telemetry';
import { ARTISTIC_MAP_STYLES } from '../../styles/artisticMapStyles';
import { ScaleBar } from '../ScaleBar';
import { CompassRose } from '../CompassRose';

interface ArtisticMapOverlaysProps {
    zoom: number;
    center: [number, number];
    telemetry: Telemetry | null;
    effectiveReplayMode: boolean;
    paperOpacityFog: number;
    paperOpacityClear: number;
    parchmentSaturation: number;
    maskPath: string;
}

export const ArtisticMapOverlays: React.FC<ArtisticMapOverlaysProps> = React.memo(({
    zoom,
    center,
    telemetry,
    effectiveReplayMode,
    paperOpacityFog,
    paperOpacityClear,
    parchmentSaturation,
    maskPath
}) => {
    const getMaskColor = (opacity: number) => {
        const val = Math.floor(opacity * 255);
        return `rgb(${val}, ${val}, ${val})`;
    };

    const isIdle = telemetry?.SimState === 'disconnected' && !effectiveReplayMode;
    const useClearOpacity = effectiveReplayMode || isIdle || telemetry?.SimState === 'inactive';
    const baseOpacity = useClearOpacity ? paperOpacityClear : paperOpacityFog;

    return (
        <>
            {/* Dual-Scale Bar */}
            <ScaleBar zoom={zoom} latitude={center[1]} />

            {/* Compass Rose */}
            <div style={{
                position: 'absolute', right: 20, bottom: 20, zIndex: 15,
                opacity: 0.8, pointerEvents: 'none',
                color: ARTISTIC_MAP_STYLES.colors.icon.compass
            }}>
                <CompassRose size={58} />
            </div>

            {/* SVG Filter Definitions */}
            <svg style={{ position: 'absolute', width: 0, height: 0 }}>
                <defs>
                    <mask id="paper-mask" maskContentUnits="userSpaceOnUse">
                        <rect x="0" y="0" width="10000" height="10000" fill={getMaskColor(baseOpacity)} />
                        {!useClearOpacity && <path d={maskPath} fill={getMaskColor(paperOpacityClear)} />}
                    </mask>
                </defs>
            </svg>

            {/* Paper Overlay */}
            <div style={{
                position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', pointerEvents: 'none',
                backgroundColor: '#f4ecd8', backgroundImage: 'url(/assets/textures/paper.jpg), radial-gradient(#d4af37 1px, transparent 1px)',
                backgroundSize: 'cover, 20px 20px', zIndex: 10, mask: 'url(#paper-mask)', WebkitMask: 'url(#paper-mask)',
                filter: `saturate(${parchmentSaturation})`
            }} />
        </>
    );
});
