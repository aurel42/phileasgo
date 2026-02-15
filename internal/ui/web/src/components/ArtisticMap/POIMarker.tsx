import React, { useMemo } from 'react';
import type { LabelCandidate } from '../../metrics/PlacementEngine';
import type { POI } from '../../hooks/usePOIs';
import { ARTISTIC_MAP_STYLES } from '../../styles/artisticMapStyles';
import { lerpColor } from '../../utils/mapUtils';
import { InlineSVG } from '../InlineSVG';
import { WaxSeal } from '../WaxSeal';
import { POIBeacon } from '../POIBeacon';

interface POIMarkerProps {
    label: LabelCandidate;
    poi: POI | undefined;
    selectedPOI: POI | null | undefined;
    currentNarratedId: string | undefined;
    preparingId: string | undefined;
    isAutoOpened: boolean;
    isReplayItem: boolean;
    zoomScale: number;
    finalX: number;
    finalY: number;
    geoX: number;
    geoY: number;
    fadeOpacity: number;
    onPOISelect: (poi: POI) => void;
}

export const POIMarker: React.FC<POIMarkerProps> = React.memo(({
    label, poi, selectedPOI, currentNarratedId, preparingId,
    isAutoOpened, isReplayItem, zoomScale, finalX, finalY, geoX, geoY,
    fadeOpacity, onPOISelect
}) => {
    const isActive = label.id === currentNarratedId;
    const isPreparing = label.id === preparingId;
    const isSelected = selectedPOI && label.id === selectedPOI.wikidata_id;
    const activeBoost = isActive ? 1.5 : (isPreparing ? 1.25 : (isSelected ? 1.4 : 1.0));

    const isCapped = (zoomScale * activeBoost) > 4.0;
    const renderScale = isCapped ? 4.0 : (zoomScale * activeBoost);
    const iconName = (poi?.icon_artistic || label.icon || 'attraction');
    const iconUrl = `/icons/${iconName}.svg`;

    const effectiveIsHistorical = label.isHistorical && !isActive && !isPreparing;

    const tDist = Math.ceil(Math.sqrt(Math.pow(finalX - geoX, 2) + Math.pow(finalY - geoY, 2)));
    const isDisplaced = !effectiveIsHistorical && tDist > 68;

    const score = label.score || 0;
    const isHero = score >= 20 && !effectiveIsHistorical;

    let iconColor = ARTISTIC_MAP_STYLES.colors.icon.copper;
    if (effectiveIsHistorical) {
        iconColor = ARTISTIC_MAP_STYLES.colors.icon.historical;
    } else {
        if (score <= 0) iconColor = ARTISTIC_MAP_STYLES.colors.icon.silver;
        else if (score >= 20) iconColor = ARTISTIC_MAP_STYLES.colors.icon.gold;
        else iconColor = lerpColor(ARTISTIC_MAP_STYLES.colors.icon.silver, ARTISTIC_MAP_STYLES.colors.icon.gold, score / 20.0);
    }

    const isDeferred = !isReplayItem && (poi?.is_deferred || poi?.badges?.includes('deferred'));
    const isLOSBlocked = poi?.los_status === 2;

    let hColor = ARTISTIC_MAP_STYLES.colors.icon.normalHalo;
    let hSize = 2;
    let hLayers = isActive ? 3 : (isPreparing ? 2 : 1);

    if (isHero) hColor = ARTISTIC_MAP_STYLES.colors.icon.gold;
    if (isSelected && !isAutoOpened) { hColor = ARTISTIC_MAP_STYLES.colors.icon.neonCyan; hLayers = 3; }
    if (score > 30 && !effectiveIsHistorical) hSize = 2 + (Math.sqrt(score / 10 - 2) - 1) * 3.5;

    let silhouette = isDeferred || isLOSBlocked;
    let outlineWeight = 1.2;
    if (poi?.badges?.includes('deep_dive')) outlineWeight = 1.45;
    let outlineColor = effectiveIsHistorical ? iconColor : ARTISTIC_MAP_STYLES.colors.icon.stroke;

    if (label.custom) {
        if (label.custom.silhouette) silhouette = true;
        if (label.custom.weight) outlineWeight = label.custom.weight;
        if (label.custom.color) outlineColor = label.custom.color;
    }

    if (silhouette) { iconColor = '#000000'; outlineColor = '#ffffff'; hColor = '#ffffff'; hSize = 3; hLayers = 1; }

    let dropShadowFilter = '';
    if (effectiveIsHistorical || (label.custom?.halo === 'none')) {
        dropShadowFilter = 'none';
    } else if (label.custom?.halo === 'organic') {
        dropShadowFilter = `drop-shadow(1px 1px 2px ${ARTISTIC_MAP_STYLES.colors.icon.organicSmudge}) drop-shadow(-1px -1px 2px ${ARTISTIC_MAP_STYLES.colors.icon.organicSmudge})`;
    } else {
        if (hLayers === 3) dropShadowFilter = `drop-shadow(0 0 ${hSize / 2}px ${hColor}) drop-shadow(0 0 ${hSize}px ${hColor}) drop-shadow(0 0 ${hSize * 2}px ${hColor})`;
        else if (hLayers === 2) dropShadowFilter = `drop-shadow(0 0 ${hSize / 2}px ${hColor}) drop-shadow(0 0 ${hSize}px ${hColor})`;
        else dropShadowFilter = `drop-shadow(0 0 ${hSize}px ${hColor})`;
    }

    const swayOut = 36; const swayIn = 24;
    const swayDir = (label.id.charCodeAt(0) % 2 === 0 ? 1 : -1);
    const startX = geoX; const startY = geoY; const endX = finalX; const endY = finalY; const dy = endY - startY;
    const cp1OutX = startX + (swayOut * swayDir); const cp1OutY = startY + (dy * 0.1); const cp2OutX = endX - (swayOut * swayDir); const cp2OutY = startY + (dy * 0.9);
    const cp1InX = startX + (swayIn * swayDir); const cp1InY = startY + (dy * 0.1); const cp2InX = endX - (swayIn * swayDir); const cp2InY = startY + (dy * 0.9);

    const rotation = useMemo(() => label.id.split('').reduce((acc, char) => acc + char.charCodeAt(0), 0) % 360, [label.id]);

    return (
        <>
            {isDisplaced && (
                <svg style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', pointerEvents: 'none', zIndex: 15, opacity: fadeOpacity * ARTISTIC_MAP_STYLES.tethers.opacity }}>
                    <path d={`M ${startX},${startY} C ${cp1OutX},${cp1OutY} ${cp2OutX},${cp2OutY} ${endX},${endY} C ${cp2InX},${cp2InY} ${cp1InX},${cp1InY} ${startX},${startY} Z`}
                        fill={ARTISTIC_MAP_STYLES.tethers.stroke} stroke={ARTISTIC_MAP_STYLES.tethers.stroke} strokeWidth={ARTISTIC_MAP_STYLES.tethers.width} strokeLinejoin="round" />
                    <circle cx={startX} cy={startY} r={ARTISTIC_MAP_STYLES.tethers.dotRadius} fill={ARTISTIC_MAP_STYLES.tethers.stroke} opacity={ARTISTIC_MAP_STYLES.tethers.dotOpacity} />
                </svg>
            )}
            {(isActive || isPreparing) && (
                <div style={{
                    position: 'absolute', left: finalX, top: finalY,
                    transform: `translate(-50%, -50%) scale(${renderScale}) rotate(${rotation}deg)`,
                    opacity: isActive ? 1 : 0.8, // Simplified Preparing opacity for now
                    pointerEvents: 'none', zIndex: isActive ? 99 : 89
                }}>
                    <WaxSeal size={label.width} />
                </div>
            )}
            <div
                onClick={(e) => { e.stopPropagation(); if (poi) onPOISelect(poi); }}
                style={{
                    position: 'absolute', left: finalX, top: finalY, width: label.width, height: label.height,
                    transform: `translate(-50%, -50%) scale(${renderScale})`,
                    opacity: (effectiveIsHistorical ? 0.5 : 1) * fadeOpacity,
                    color: iconColor, cursor: 'pointer', pointerEvents: 'auto',
                    filter: dropShadowFilter, zIndex: isActive ? 100 : (isPreparing ? 90 : 15)
                }}
            >
                <InlineSVG src={iconUrl} style={{ '--stamped-stroke': outlineColor, '--stamped-width': `${outlineWeight}px` } as any} className="stamped-icon" />
            </div>
            {poi?.is_msfs_poi && !isReplayItem && (
                <div style={{
                    position: 'absolute', left: finalX, top: finalY, width: 18 * renderScale, height: 18 * renderScale,
                    transform: `translate(${(label.width / 2 - 4) * renderScale}px, ${(-label.height / 2 + 4) * renderScale}px) translate(-50%, -50%)`,
                    color: ARTISTIC_MAP_STYLES.colors.icon.gold, zIndex: isActive ? 101 : (isPreparing ? 91 : 16),
                    pointerEvents: 'none', opacity: fadeOpacity
                }}>
                    <InlineSVG src="/icons/star.svg" style={{ '--stamped-stroke': ARTISTIC_MAP_STYLES.colors.icon.stroke, '--stamped-width': '1.2px' } as any} className="stamped-icon" />
                </div>
            )}
            {label.markerLabel && label.markerLabelPos && !isCapped && (
                <div className="role-label" style={{
                    position: 'absolute',
                    left: finalX + ((label.markerLabelPos.x - (label.finalX ?? 0)) * zoomScale),
                    top: finalY + ((label.markerLabelPos.y - (label.finalY ?? 0)) * zoomScale),
                    transform: `translate(-50%, -50%) scale(${zoomScale})`,
                    fontSize: '17px', opacity: fadeOpacity, pointerEvents: 'none', zIndex: 25,
                    textShadow: ARTISTIC_MAP_STYLES.colors.shadows.atmosphere, whiteSpace: 'nowrap'
                }}>
                    {label.markerLabel.text}
                </div>
            )}
            {(isActive || isPreparing || poi?.has_balloon) && poi?.beacon_color && !isReplayItem && (
                <POIBeacon x={finalX} y={finalY} color={poi.beacon_color} size={12} zoomScale={renderScale} iconSize={label.height} />
            )}
        </>
    );
});
