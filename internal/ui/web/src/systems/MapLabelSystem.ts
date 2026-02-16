import maplibregl from 'maplibre-gl';
import type { IMapSystem, MapContext, SystemState } from './types';
import { PlacementEngine } from '../metrics/PlacementEngine';
import { measureText, adjustFont, getFontFromClass } from '../utils/mapFonts';
import type { POI } from '../hooks/usePOIs';
import { labelService } from '../services/labelService';

export class MapLabelSystem implements IMapSystem {
    private engine: PlacementEngine;
    private mapRef: React.MutableRefObject<maplibregl.Map | null>;

    // State Tracking
    private lastPoiCount: number = 0;
    private lastLabelsJson: string = "";
    private lastPlacementView: { lng: number, lat: number, zoom: number } | null = null;
    private lastNarratedId: string | undefined = undefined;
    private lastPreparingId: string | undefined = undefined;

    // Labeling State
    private labeledPoiIds = new Set<string>();
    private failedPoiLabelIds = new Set<string>();

    // Data Fetching
    private lastFetchTime: number = 0;
    private fetchInterval: number = 2000;

    constructor(mapRef: React.MutableRefObject<maplibregl.Map | null>) {
        this.engine = new PlacementEngine();
        this.mapRef = mapRef;
    }

    reset() {
        this.engine.clear();
        this.engine.resetCache();
        this.lastPoiCount = 0;
        this.lastLabelsJson = "";
        this.lastPlacementView = null;
        this.lastNarratedId = undefined;
        this.lastPreparingId = undefined;
        this.labeledPoiIds.clear();
        this.failedPoiLabelIds.clear();
    }

    update(_dt: number, ctx: MapContext, state: SystemState) {
        if (!ctx.styleLoaded || !ctx.fontsLoaded || !this.mapRef.current) return;
        const m = this.mapRef.current;
        const zoom = state.frame.zoom;
        const center = state.frame.center;

        // Data Change Detection
        const currentPois = ctx.pois;
        // Settlement Accumulation
        if (currentPois.length > 0) {
            const settlementCats = new Set(ctx.settlementCategories.map(c => c.toLowerCase()));
            currentPois.forEach(p => {
                if (settlementCats.has(p.category?.toLowerCase()) && !state.accumulatedSettlements.has(p.wikidata_id)) {
                    state.accumulatedSettlements.set(p.wikidata_id, {
                        id: p.wikidata_id,
                        lat: p.lat,
                        lon: p.lon,
                        name: p.name_en,
                        category: p.category.toLowerCase(),
                        pop: p.score // Mapping score to population for sizing
                    });
                }
            });
        }

        // Remote Settlement Fetching (Throttled)
        const now = performance.now();
        if (ctx.mode !== 'REPLAY' && (now - this.lastFetchTime > this.fetchInterval)) {
            const b = m.getBounds();
            if (b) {
                // Determine visible bbox
                this.lastFetchTime = now;
                labelService.fetchLabels({
                    bbox: [b.getSouth(), b.getWest(), b.getNorth(), b.getEast()],
                    ac_lat: center[1], ac_lon: center[0], heading: state.frame.heading, zoom: zoom
                }).then(newLabels => {
                    newLabels.forEach(l => state.accumulatedSettlements.set(l.id, l));
                }).catch(e => console.error("Label Sync Failed:", e));
            }
        }

        const labelsJson = JSON.stringify(Array.from(state.accumulatedSettlements.keys()));
        const dataChanged = currentPois.length !== this.lastPoiCount || labelsJson !== this.lastLabelsJson;

        // View Change Detection (Thresholds: 0.05 zoom, 0.0001 deg pos ~11m)
        let viewChanged = !this.lastPlacementView ||
            Math.abs(this.lastPlacementView.zoom - zoom) > 0.05 ||
            Math.abs(this.lastPlacementView.lng - center[0]) > 0.0001 ||
            Math.abs(this.lastPlacementView.lat - center[1]) > 0.0001;

        // Status Change Detection
        const currentNarratedId = ctx.narratorStatus?.current_poi?.wikidata_id;
        const preparingId = ctx.narratorStatus?.preparing_poi?.wikidata_id;
        const statusChanged = currentNarratedId !== this.lastNarratedId || preparingId !== this.lastPreparingId;

        const needsSnap = viewChanged || statusChanged || dataChanged;

        if (needsSnap) {
            if (ctx.mode === 'TRANSITION') this.engine.resetCache();
            this.engine.clear();

            this.registerSettlements(state.accumulatedSettlements);

            let champion: POI | null = null;
            if (ctx.mode === 'REPLAY') {
                this.registerReplayPOIs(ctx);
            } else {
                champion = this.selectChampion(ctx, currentPois);
                this.registerLivePOIs(currentPois, champion);
            }

            // Compute
            const labels = this.engine.compute((lat, lon) => {
                const pos = m.project([lon, lat]);
                return { x: pos.x, y: pos.y };
            }, ctx.mapWidth, ctx.mapHeight, zoom);

            // Champion Check
            if (champion) {
                const placedChamp = labels.find(l => l.id === champion!.wikidata_id);
                if (placedChamp?.markerLabel) {
                    if (placedChamp.markerLabelPos) this.labeledPoiIds.add(champion.wikidata_id);
                    else this.failedPoiLabelIds.add(champion.wikidata_id);
                }
            }

            // Update State
            this.lastPoiCount = currentPois.length;
            this.lastLabelsJson = labelsJson;
            this.lastPlacementView = { lng: center[0], lat: center[1], zoom: zoom };
            this.lastNarratedId = currentNarratedId;
            this.lastPreparingId = preparingId;

            state.frame.labels = labels;
        }
    }

    private registerSettlements(settlements: Map<string, any>) {
        const cityFont = adjustFont(getFontFromClass('role-title'), -4);
        const townFont = adjustFont(getFontFromClass('role-header'), -4);
        const villageFont = adjustFont(getFontFromClass('role-text-lg'), -4);

        Array.from(settlements.values()).forEach((l: any) => {
            let role = villageFont; let tierName: 'city' | 'town' | 'village' = 'village';
            if (l.category === 'city') { tierName = 'city'; role = cityFont; }
            else if (l.category === 'town') { tierName = 'town'; role = townFont; }

            let text = l.name.split('(')[0].split(',')[0].split('/')[0].trim();
            if (role.uppercase) text = text.toUpperCase();
            const dims = measureText(text, role.font, role.letterSpacing);

            this.engine.register({
                id: l.id, lat: l.lat, lon: l.lon, text, tier: tierName,
                width: dims.width, height: dims.height, type: 'settlement',
                score: l.pop || 0, isHistorical: false, size: 'L'
            });
        });
    }

    private selectChampion(ctx: MapContext, pois: POI[]): POI | null {
        const settlementCatSet = new Set(ctx.settlementCategories.map(c => c.toLowerCase()));
        let champion: POI | null = null;

        pois.forEach(p => {
            if (settlementCatSet.has(p.category?.toLowerCase())) return;
            const normalizedName = p.name_en.split('(')[0].split(',')[0].split('/')[0].trim();
            if (normalizedName.length > 24) return;

            const isHistorical = !!(p.last_played && p.last_played !== "0001-01-01T00:00:00Z");
            if (isHistorical) return;

            if (p.score >= 10 && !this.failedPoiLabelIds.has(p.wikidata_id) && !this.labeledPoiIds.has(p.wikidata_id)) {
                if (!champion || (p.score > champion.score)) champion = p;
            }
        });
        return champion;
    }

    private registerLivePOIs(pois: POI[], champion: POI | null) {
        const markerLabelFont = adjustFont(getFontFromClass('role-label'), 2);

        pois.forEach(p => {
            const isChampion = champion && p.wikidata_id === champion.wikidata_id;
            const needsMarkerLabel = isChampion || this.labeledPoiIds.has(p.wikidata_id);
            const isHistorical = !!(p.last_played && p.last_played !== "0001-01-01T00:00:00Z");

            let markerLabel = undefined;
            if (needsMarkerLabel) {
                let text = p.name_en.split('(')[0].split(',')[0].split('/')[0].trim();
                if (markerLabelFont.uppercase) text = text.toUpperCase();
                const dims = measureText(text, markerLabelFont.font, markerLabelFont.letterSpacing);
                markerLabel = { text, width: dims.width, height: dims.height };
            }

            this.engine.register({
                id: p.wikidata_id, lat: p.lat, lon: p.lon, text: "", tier: 'village',
                score: p.score || 0, width: 26, height: 26, type: 'poi',
                isHistorical, size: p.size as any, icon: p.icon,
                visibility: p.visibility, markerLabel
            });
        });
    }

    private registerReplayPOIs(ctx: MapContext) {
        const markerLabelFont = adjustFont(getFontFromClass('role-label'), 2);
        const simulatedElapsed = ctx.progress * ctx.totalTripTime;
        const processedIds = new Set<string>();

        ctx.validEvents.forEach(e => {
            const eid = e.metadata?.poi_id || e.metadata?.qid;
            if (!eid || processedIds.has(eid)) return;
            processedIds.add(eid);

            const discoveryTime = ctx.poiDiscoveryTimes.get(eid);
            if (discoveryTime != null && (discoveryTime - ctx.firstEventTime) > simulatedElapsed) return;

            const lat = e.metadata.poi_lat ? parseFloat(e.metadata.poi_lat) : e.lat;
            const lon = e.metadata.poi_lon ? parseFloat(e.metadata.poi_lon) : e.lon;
            if (!lat || !lon) return;

            const name = e.title || e.metadata.poi_name || 'Point of Interest';
            const score = e.metadata.poi_score ? parseFloat(e.metadata.poi_score) : 30;

            let markerLabel = undefined;
            if (score >= 10) {
                let text = name.split('(')[0].split(',')[0].split('/')[0].trim();
                if (markerLabelFont.uppercase) text = text.toUpperCase();
                const dims = measureText(text, markerLabelFont.font, markerLabelFont.letterSpacing);
                markerLabel = { text, width: dims.width, height: dims.height };
            }

            this.engine.register({
                id: eid, lat, lon, text: "", tier: 'village', score,
                width: 26, height: 26, type: 'poi', isHistorical: false,
                size: (e.metadata.poi_size || 'M') as any,
                icon: e.metadata.icon_artistic || e.metadata.icon || 'attraction',
                markerLabel
            });
        });
    }
}
