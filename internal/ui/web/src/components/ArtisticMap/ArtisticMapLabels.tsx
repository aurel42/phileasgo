import React from 'react';
import type maplibregl from 'maplibre-gl';
import type { LabelCandidate } from '../../metrics/PlacementEngine';
import type { POI } from '../../hooks/usePOIs';
import { POIMarker } from './POIMarker';
import { SettlementMarker } from './SettlementMarker';

interface ArtisticMapLabelsProps {
    map: React.MutableRefObject<maplibregl.Map | null>;
    labels: LabelCandidate[];
    accumulatedPois: React.MutableRefObject<Map<string, POI>>;
    labelAppearanceRef: React.MutableRefObject<Map<string, number>>;
    selectedPOI: POI | null | undefined;
    currentNarratedId: string | undefined;
    preparingId: string | undefined;
    isAutoOpened: boolean;
    effectiveReplayMode: boolean;
    progress: number;
    poiDiscoveryTimes: Map<string, number>;
    replayFirstTime: number;
    replayTotalTime: number;
    currentZoom: number;
    onPOISelect: (poi: POI) => void;
}

export const ArtisticMapLabels: React.FC<ArtisticMapLabelsProps> = React.memo(({
    map, labels, accumulatedPois, labelAppearanceRef, selectedPOI,
    currentNarratedId, preparingId, isAutoOpened, effectiveReplayMode,
    progress, poiDiscoveryTimes, replayFirstTime, replayTotalTime, currentZoom,
    onPOISelect
}) => {
    const m = map.current;
    if (!m) return null;

    return (
        <div style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '100%', pointerEvents: 'none', zIndex: 20 }}>
            {labels.map(l => {
                // Replay Discovery Filter
                if (effectiveReplayMode && l.type === 'poi') {
                    const discoveryTime = poiDiscoveryTimes.get(l.id);
                    if (discoveryTime != null) {
                        const eventElapsed = discoveryTime - replayFirstTime;
                        const simulatedElapsed = progress * replayTotalTime;
                        if (progress < 0.999 && eventElapsed > simulatedElapsed) return null;
                    }
                }

                const geoPos = m.project([l.lon, l.lat]);
                let zoomScale = l.placedZoom != null ? Math.pow(2, currentZoom - l.placedZoom) : 1;
                if (l.type === 'settlement') zoomScale = Math.min(zoomScale, 1.0);

                const offsetX = ((l.finalX ?? 0) - (l.trueX ?? 0)) * zoomScale;
                const offsetY = ((l.finalY ?? 0) - (l.trueY ?? 0)) * zoomScale;
                const finalX = geoPos.x + offsetX;
                const finalY = geoPos.y + offsetY;

                const now = Date.now();
                if (!labelAppearanceRef.current.has(l.id)) {
                    labelAppearanceRef.current.set(l.id, now);
                }
                const start = labelAppearanceRef.current.get(l.id)!;
                const fadeOpacity = Math.min(1, (now - start) / 2000);

                if (l.type === 'poi') {
                    const poi = accumulatedPois.current.get(l.id);
                    return (
                        <POIMarker
                            key={l.id}
                            label={l}
                            poi={poi}
                            selectedPOI={selectedPOI}
                            currentNarratedId={currentNarratedId}
                            preparingId={preparingId}
                            isAutoOpened={isAutoOpened}
                            isReplayItem={effectiveReplayMode}
                            zoomScale={zoomScale}
                            finalX={finalX}
                            finalY={finalY}
                            geoX={geoPos.x}
                            geoY={geoPos.y}
                            fadeOpacity={fadeOpacity}
                            onPOISelect={onPOISelect}
                        />
                    );
                }

                return (
                    <SettlementMarker
                        key={l.id}
                        label={l}
                        finalX={finalX}
                        finalY={finalY}
                        zoomScale={zoomScale}
                        fadeOpacity={fadeOpacity}
                    />
                );
            })}
        </div>
    );
});
