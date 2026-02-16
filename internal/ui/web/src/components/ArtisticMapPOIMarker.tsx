import React from 'react';
import type { LabelCandidate } from '../metrics/PlacementEngine';
import type { POI } from '../hooks/usePOIs';
import { ARTISTIC_MAP_STYLES } from '../styles/artisticMapStyles';
import { lerpColor } from '../utils/artisticMapUtils';
import { InlineSVG } from './InlineSVG';
import { WaxSeal } from './WaxSeal';
import { POIBeacon } from './POIBeacon';

const DEBUG_FLOURISHES = false;

export interface ArtisticMapPOIMarkerProps {
    label: LabelCandidate;
    poi: POI | undefined;
    geoPos: { x: number; y: number };
    finalX: number;
    finalY: number;
    zoomScale: number;
    fadeOpacity: number;
    selectedPOI: POI | null | undefined;
    currentNarratedId: string | undefined;
    preparingId: string | undefined;
    isAutoOpened: boolean;
    isReplayItem: boolean;
    preparingOpacity: number;
    onPOISelect: (poi: POI) => void;
}

export const ArtisticMapPOIMarker: React.FC<ArtisticMapPOIMarkerProps> = ({
    label: l, poi, geoPos, finalX, finalY, zoomScale, fadeOpacity,
    selectedPOI, currentNarratedId, preparingId, isAutoOpened, isReplayItem,
    preparingOpacity, onPOISelect
}) => {
    // 2. Halo Properties
    const isSelected = selectedPOI && l.id === selectedPOI.wikidata_id;
    const isActive = l.id === currentNarratedId;
    const isPreparing = l.id === preparingId;
    const activeBoost = isActive ? 1.5 : (isPreparing ? 1.25 : (isSelected ? 1.4 : 1.0));

    const isCapped = (zoomScale * activeBoost) > 4.0;
    const renderScale = isCapped ? 4.0 : (zoomScale * activeBoost);
    const iconName = (poi?.icon_artistic || l.icon || 'attraction');
    const iconUrl = `/icons/${iconName}.svg`;

    // Semantic Logic Simplification: Playing or Preparing items are NEVER treated as historic
    const effectiveIsHistorical = l.isHistorical && !isActive && !isPreparing;

    // Tether logic: Historic items never get tethers, regardless of distance.
    const tTx = geoPos.x;
    const tTy = geoPos.y;
    const tDx = finalX - tTx;
    const tDy = finalY - tTy;
    const tDist = Math.ceil(Math.sqrt(tDx * tDx + tDy * tDy));
    const isDisplaced = !effectiveIsHistorical && tDist > 68;

    const score = l.score || 0;
    const isHero = score >= 20 && !effectiveIsHistorical;

    // 1. Icon Fill Color: Strictly Score-based (Silver -> Gold) unless Historical
    let iconColor = ARTISTIC_MAP_STYLES.colors.icon.copper;
    if (effectiveIsHistorical) {
        iconColor = ARTISTIC_MAP_STYLES.colors.icon.historical;
    } else {
        if (score <= 0) {
            iconColor = ARTISTIC_MAP_STYLES.colors.icon.silver;
        } else if (score >= 20) {
            iconColor = ARTISTIC_MAP_STYLES.colors.icon.gold;
        } else {
            const t = score / 20.0;
            iconColor = lerpColor(ARTISTIC_MAP_STYLES.colors.icon.silver, ARTISTIC_MAP_STYLES.colors.icon.gold, t);
        }
    }

    // 2. Halo Properties (Continued)
    const isDeferred = !isReplayItem && (poi?.is_deferred || poi?.badges?.includes('deferred'));
    const isLOSBlocked = poi?.los_status === 2;

    let hColor = ARTISTIC_MAP_STYLES.colors.icon.normalHalo; // Paper White
    let hSize = 2;
    let hLayers = isActive ? 3 : (isPreparing ? 2 : 1);

    if (isHero) {
        hColor = ARTISTIC_MAP_STYLES.colors.icon.gold;
    }

    if (isSelected && !isAutoOpened) {
        hColor = ARTISTIC_MAP_STYLES.colors.icon.neonCyan;
        hLayers = 3;
    }

    if (score > 30 && !effectiveIsHistorical) {
        // Non-linear scaling: 2px at 30, ~5px at 150 (X=3.5)
        hSize = 2 + (Math.sqrt(score / 10 - 2) - 1) * 3.5;
    }

    let silhouette = false;
    if (isDeferred || isLOSBlocked) {
        silhouette = true;
    }

    let outlineWeight = 1.2; // Optimized from 0.8 (+50%) for better ink clarity

    const isDeepDive = poi?.badges?.includes('deep_dive');
    if (isDeepDive) {
        outlineWeight = 1.45; // "Heavy Ink" style for deep dive content (-20% from 1.8)
    }

    let outlineColor = effectiveIsHistorical ? iconColor : ARTISTIC_MAP_STYLES.colors.icon.stroke;

    if (l.custom) {
        if (l.custom.silhouette) silhouette = true;
        if (l.custom.weight) outlineWeight = l.custom.weight;
        if (l.custom.color) outlineColor = l.custom.color;
    }

    if (silhouette) {
        iconColor = '#000000';
        outlineColor = '#ffffff';
        hColor = '#ffffff';
        hSize = 3;
        hLayers = 1;
    }

    // Filter mapping for Halos (applying dynamic size and layers)
    let dropShadowFilter = '';
    if (effectiveIsHistorical || (l.custom?.halo === 'none')) {
        dropShadowFilter = 'none';
    } else if (l.custom?.halo === 'organic' || (l.id.startsWith('dbg-2') && DEBUG_FLOURISHES)) {
        dropShadowFilter = `drop-shadow(1px 1px 2px ${ARTISTIC_MAP_STYLES.colors.icon.organicSmudge}) drop-shadow(-1px -1px 2px ${ARTISTIC_MAP_STYLES.colors.icon.organicSmudge})`;
    } else {
        // Standard or Special (Neon, Gold, Selected)
        if (hLayers === 3) {
            dropShadowFilter = `drop-shadow(0 0 ${hSize / 2}px ${hColor}) drop-shadow(0 0 ${hSize}px ${hColor}) drop-shadow(0 0 ${hSize * 2}px ${hColor})`;
        } else if (hLayers === 2) {
            dropShadowFilter = `drop-shadow(0 0 ${hSize / 2}px ${hColor}) drop-shadow(0 0 ${hSize}px ${hColor})`;
        } else {
            dropShadowFilter = `drop-shadow(0 0 ${hSize}px ${hColor})`;
        }
    }

    // Silhouette Logic
    if (l.custom?.silhouette) silhouette = true;

    if (silhouette) {
        // Empty: moved to InlineSVG filter for better halo preservation
    }


    const swayOut = 36;
    const swayIn = 24;
    // Deterministic sway direction based on ID to keep it stable but organic
    const swayDir = (l.id.charCodeAt(0) % 2 === 0 ? 1 : -1);
    const startX = geoPos.x;
    const startY = geoPos.y;
    const endX = finalX;
    const endY = finalY;
    const dy = endY - startY;

    // Calligraphic stroke: Two Bezier curves (Outer/Inner) forming a filled shape that's bulky in the center
    const cp1OutX = startX + (swayOut * swayDir);
    const cp1OutY = startY + (dy * 0.1);
    const cp2OutX = endX - (swayOut * swayDir);
    const cp2OutY = startY + (dy * 0.9);

    const cp1InX = startX + (swayIn * swayDir);
    const cp1InY = startY + (dy * 0.1);
    const cp2InX = endX - (swayIn * swayDir);
    const cp2InY = startY + (dy * 0.9);

    // Wax seal opacity: for active narration it's 1, for preparing it's the pre-computed value
    const waxSealOpacity = l.id === currentNarratedId ? 1 : preparingOpacity;

    return (
        <React.Fragment key={l.id}>
            {isDisplaced && (
                <svg style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', pointerEvents: 'none', zIndex: 15, opacity: fadeOpacity * ARTISTIC_MAP_STYLES.tethers.opacity }}>
                    <path d={`M ${startX},${startY} C ${cp1OutX},${cp1OutY} ${cp2OutX},${cp2OutY} ${endX},${endY} C ${cp2InX},${cp2InY} ${cp1InX},${cp1InY} ${startX},${startY} Z`}
                        fill={ARTISTIC_MAP_STYLES.tethers.stroke}
                        stroke={ARTISTIC_MAP_STYLES.tethers.stroke}
                        strokeWidth={ARTISTIC_MAP_STYLES.tethers.width}
                        strokeLinejoin="round" />
                    <circle cx={startX} cy={startY} r={ARTISTIC_MAP_STYLES.tethers.dotRadius} fill={ARTISTIC_MAP_STYLES.tethers.stroke} opacity={ARTISTIC_MAP_STYLES.tethers.dotOpacity} />
                </svg>
            )}
            {(l.id === currentNarratedId || l.id === preparingId) && (
                <div style={{
                    position: 'absolute', left: finalX, top: finalY,
                    // Stable random rotation based on ID bytes
                    transform: `translate(-50%, -50%) scale(${renderScale}) rotate(${(l.id.split('').reduce((acc, char) => acc + char.charCodeAt(0), 0) % 360)}deg)`,
                    opacity: waxSealOpacity,
                    pointerEvents: 'none',
                    zIndex: l.id === currentNarratedId ? 99 : 89
                }}>
                    <WaxSeal size={l.width} />
                </div>
            )}
            <div
                onClick={(e) => {
                    e.stopPropagation();
                    if (poi && onPOISelect) onPOISelect(poi);
                }}
                style={{
                    position: 'absolute', left: finalX, top: finalY, width: l.width, height: l.height,
                    transform: `translate(-50%, -50%) scale(${renderScale})`,
                    opacity: (effectiveIsHistorical ? 0.5 : 1) * fadeOpacity,
                    color: iconColor, cursor: 'pointer', pointerEvents: 'auto',
                    // Use drop-shadow filter for true shape contour ("Halo")
                    filter: dropShadowFilter,
                    zIndex: l.id === currentNarratedId ? 100 : (l.id === preparingId ? 90 : 15)
                }}
            >
                <InlineSVG
                    src={iconUrl}
                    style={{
                        // @ts-ignore - custom CSS variables for the stamped-icon class
                        '--stamped-stroke': outlineColor,
                        '--stamped-width': `${outlineWeight}px`
                    }}
                    className="stamped-icon"
                />
            </div>
            {poi?.is_msfs_poi && !isReplayItem && (
                <div style={{
                    position: 'absolute',
                    left: finalX,
                    top: finalY,
                    width: 18 * renderScale,
                    height: 18 * renderScale,
                    // Position star at top-right of the scaled icon
                    transform: `translate(${(l.width / 2 - 4) * renderScale}px, ${(-l.height / 2 + 4) * renderScale}px) translate(-50%, -50%)`,
                    color: ARTISTIC_MAP_STYLES.colors.icon.gold,
                    zIndex: l.id === currentNarratedId ? 101 : (l.id === preparingId ? 91 : 16),
                    pointerEvents: 'none',
                    opacity: fadeOpacity
                }}>
                    <InlineSVG
                        src="/icons/star.svg"
                        style={{
                            // @ts-ignore
                            '--stamped-stroke': ARTISTIC_MAP_STYLES.colors.icon.stroke,
                            '--stamped-width': '1.2px'
                        }}
                        className="stamped-icon"
                    />
                </div>
            )}

            {l.markerLabel && l.markerLabelPos && !isCapped && (
                <div
                    className="role-label"
                    style={{
                        position: 'absolute',
                        // Project the marker label anchor displacement relative to the dynamic final coordinate
                        left: finalX + ((l.markerLabelPos.x - (l.finalX ?? 0)) * zoomScale),
                        top: finalY + ((l.markerLabelPos.y - (l.finalY ?? 0)) * zoomScale),
                        transform: `translate(-50%, -50%) scale(${zoomScale})`,
                        fontSize: '17px', // Match markerLabelFont adjustment (+2)
                        opacity: fadeOpacity,
                        pointerEvents: 'none',
                        zIndex: 25,
                        textShadow: ARTISTIC_MAP_STYLES.colors.shadows.atmosphere,
                        whiteSpace: 'nowrap'
                    }}
                >
                    {l.markerLabel.text}
                </div>
            )}

            {/* POI Balloon Badge: render for playing/preparing (render-scoped) OR all other POIs via has_balloon flag */}
            {(isActive || isPreparing || poi?.has_balloon) && poi?.beacon_color && !isReplayItem && (
                <POIBeacon
                    x={finalX}
                    y={finalY}
                    color={poi.beacon_color}
                    size={12}
                    zoomScale={renderScale}
                    iconSize={l.height}
                />
            )}
        </React.Fragment>
    );
};
