import { useEffect, useState, useMemo } from 'react';
import type { Map } from 'maplibre-gl';
import { PlacementEngine, type LabelCandidate } from '../metrics/PlacementEngine';
import { measureText } from '../metrics/text';
import type { POI } from './usePOIs';

export function useLabelPlacement(
    map: Map | null,
    settlements: POI[],
    pois: POI[],
    zoom: number
) {
    const [visibleLabels, setVisibleLabels] = useState<LabelCandidate[]>([]);

    const engine = useMemo(() => new PlacementEngine(), []);

    useEffect(() => {
        if (!map) return;

        engine.clear();

        // 1. Register Settlements
        settlements.forEach(f => {
            if (!f.lat || !f.lon) return;
            const tier = (f.category as string || 'town').toLowerCase();
            let font = "16px 'Pinyon Script'";

            if (tier === 'city') {
                font = "bold 20px 'IM Fell DW Pica'";
            } else if (tier === 'town') {
                font = "18px 'Pinyon Script'";
            } else {
                font = "14px 'Pinyon Script'";
            }

            const text = f.name_user || f.name_en || "Unknown";
            const dims = measureText(text, font);

            engine.register({
                id: f.wikidata_id || `${f.lat.toFixed(5)}-${f.lon.toFixed(5)}`,
                lat: f.lat,
                lon: f.lon,
                text: text,
                tier: tier as any,
                score: f.score || 0,
                width: dims.width,
                height: dims.height,
                type: 'settlement',
                isHistorical: false, // Settlements are always active fixtures
                size: 'L' // Default size for sorting
            });
        });

        // 2. Register POIs
        pois.forEach(p => {
            // Basic validity check
            if (!p.lat || !p.lon) return;

            // POIs are Icon-Only (No Text)
            // Dimensions based on Size (S=16, M=20, L=24, XL=28) - approximation
            let sizePx = 20;
            const size = (p.size as 'S' | 'M' | 'L' | 'XL') || 'S';
            if (size === 'S') sizePx = 16;
            if (size === 'M') sizePx = 20;
            if (size === 'L') sizePx = 24;
            if (size === 'XL') sizePx = 28;

            // Check if historical
            const isHistorical = !!(p.last_played && p.last_played !== "0001-01-01T00:00:00Z");

            engine.register({
                id: p.wikidata_id,
                lat: p.lat,
                lon: p.lon,
                text: "", // No text for POIs
                tier: 'village', // Ignored for POIs
                score: p.score || 0,
                width: sizePx,
                height: sizePx,
                type: 'poi',
                isHistorical: isHistorical,
                size: size
            });
        });

        // Compute
        const projector = (lat: number, lon: number) => {
            if (!map) return { x: 0, y: 0 };
            const p = map.project([lon, lat]);
            return { x: p.x, y: p.y };
        };

        const result = engine.compute(projector);
        setVisibleLabels(result);

    }, [map, settlements, pois, zoom, engine]);

    return visibleLabels;
}
