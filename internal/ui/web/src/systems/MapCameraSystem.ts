import maplibregl from 'maplibre-gl';
import type { Feature } from 'geojson';
import type { IMapSystem, MapContext, SystemState } from './types';
import { interpolatePositionFromEvents } from '../utils/replay';
import { rayToEdge, maskToPath } from '../utils/mapGeometry';

export class MapCameraSystem implements IMapSystem {
    private lockedCenter: [number, number] | null = null;
    private lockedZoom: number = -1;
    private lockedOffset: [number, number] = [0, 0];
    private prevSimState: string | undefined = undefined;
    private lastMaskData: any = null;
    private mapRef: React.MutableRefObject<maplibregl.Map | null>;

    constructor(mapRef: React.MutableRefObject<maplibregl.Map | null>) {
        this.mapRef = mapRef;
    }

    reset() {
        this.lockedCenter = null;
        this.lockedZoom = -1;
        this.lockedOffset = [0, 0];
        this.prevSimState = undefined;
        this.lastMaskData = null;
    }

    update(_dt: number, ctx: MapContext, state: SystemState) {
        const m = this.mapRef.current;
        if (!m) return;

        const t = ctx.telemetry;
        const acState = this.getAircraftState(ctx);

        // Update basic frame data
        state.frame.heading = acState.heading;
        state.frame.aircraftX = 0; // Will be updated after projection
        state.frame.aircraftY = 0;
        state.frame.agl = t ? t.AltitudeAGL : 0;

        // Handle State Transitions
        const currentSimState = t?.SimState || 'disconnected';
        const stateTransition = this.prevSimState !== currentSimState;
        this.prevSimState = currentSimState;

        // Background Mask Fetching (Async, fire-and-forget style for now)
        if (currentSimState === 'active' && !this.lastMaskData) {
            const bounds = m.getBounds();
            if (bounds) {
                fetch(`/api/map/visibility-mask?bounds=${bounds.getNorth()},${bounds.getEast()},${bounds.getSouth()},${bounds.getWest()}&resolution=20`)
                    .then(r => r.ok ? r.json() : null)
                    .then(data => { if (data) this.lastMaskData = data; })
                    .catch(e => console.error("Mask fetch failed", e));
            }
        }
        if (this.lastMaskData) {
            state.frame.maskPath = maskToPath(this.lastMaskData, m);
        }

        // Camera Logic
        let needsRecenter = !this.lockedCenter || stateTransition;

        // Smart Offset Check (only if strict flight mode)
        if (this.lockedCenter && !needsRecenter && ctx.mode === 'FLIGHT') {
            needsRecenter = this.checkSmartOffset(m, acState, ctx.mapWidth, ctx.mapHeight, ctx.narratorStatus?.current_poi);
        }

        // Force snap if we just entered replay mode
        if (ctx.mode === 'REPLAY' && stateTransition) {
            needsRecenter = true;
        }

        if (needsRecenter) {
            this.recalculateCamera(m, ctx, acState, stateTransition, currentSimState);
        }

        // Sync to Map (or read from map if user interacting/easing)
        if (ctx.mode === 'REPLAY') {
            if (m.isEasing()) {
                // If map is animating, we follow the map
                const c = m.getCenter();
                this.lockedCenter = [c.lng, c.lat];
                this.lockedZoom = m.getZoom();
                this.lockedOffset = [0, 0];
            } else if (needsRecenter) {
                // Snap!
                m.easeTo({
                    center: this.lockedCenter as maplibregl.LngLatLike,
                    zoom: this.lockedZoom,
                    offset: this.lockedOffset,
                    duration: 0
                });
            }
        } else if (needsRecenter) {
            m.easeTo({
                center: this.lockedCenter as maplibregl.LngLatLike,
                zoom: this.lockedZoom,
                offset: this.lockedOffset,
                duration: 0
            });
        }

        // Update Frame with final calculated values
        if (this.lockedCenter) {
            state.frame.center = this.lockedCenter;
            state.frame.zoom = this.lockedZoom;
            state.frame.offset = this.lockedOffset;

            // Update projected aircraft position for UI overlay
            const aircraftPos = m.project([acState.lon, acState.lat]);
            state.frame.aircraftX = Math.round(aircraftPos.x);
            state.frame.aircraftY = Math.round(aircraftPos.y);

            // Update Bearing Line
            this.updateBearingLine(m, acState, ctx.mode === 'REPLAY');
        }
    }

    private getAircraftState(ctx: MapContext) {
        if (ctx.mode === 'REPLAY' && ctx.validEvents.length >= 2) {
            const interp = interpolatePositionFromEvents(ctx.validEvents, ctx.progress);
            return { lat: interp.position[0], lon: interp.position[1], heading: interp.heading };
        }

        // Use random start location if we are in IDLE (disconnected) mode
        if (ctx.mode === 'IDLE' && ctx.randomLocation) {
            return { lat: ctx.randomLocation[1], lon: ctx.randomLocation[0], heading: 0 };
        }

        return {
            lat: ctx.telemetry?.Latitude || 0,
            lon: ctx.telemetry?.Longitude || 0,
            heading: ctx.telemetry?.Heading || 0
        };
    }

    private checkSmartOffset(m: maplibregl.Map, acState: any, w: number, h: number, playingPoi: any): boolean {
        const currentPos: [number, number] = [acState.lon, acState.lat];
        const aircraftOnMap = m.project(currentPos);
        const hdgRad = acState.heading * (Math.PI / 180);
        const adx = Math.sin(hdgRad);
        const ady = -Math.cos(hdgRad);

        const distAhead = rayToEdge(aircraftOnMap.x, aircraftOnMap.y, adx, ady, w, h);
        const distBehind = rayToEdge(aircraftOnMap.x, aircraftOnMap.y, -adx, -ady, w, h);

        if (distBehind > distAhead) return true;

        if (playingPoi) {
            const bounds = m.getBounds();
            if (bounds && !bounds.contains([playingPoi.lon, playingPoi.lat])) return true;
        }
        return false;
    }

    private recalculateCamera(m: maplibregl.Map, ctx: MapContext, acState: any, transition: boolean, simState: string) {
        const isIdle = simState === 'disconnected' && ctx.mode !== 'REPLAY';
        const offsetPx = isIdle ? 0 : Math.min(ctx.mapWidth, ctx.mapHeight) * 0.35;
        const hdgRad = acState.heading * (Math.PI / 180);
        const dx = offsetPx * Math.sin(hdgRad);
        const dy = -offsetPx * Math.cos(hdgRad);

        // Target Center & Zoom
        if (isIdle) {
            if (ctx.randomLocation) {
                this.lockedCenter = ctx.randomLocation;
            } else {
                this.lockedCenter = [0, 0];
            }
            this.lockedOffset = [0, 0];
        } else {
            this.lockedCenter = [acState.lon, acState.lat];
            this.lockedOffset = [-dx, -dy];
        }

        // Zoom Logic
        let newZoom = (this.lockedZoom === -1) ? ctx.zoom : this.lockedZoom;

        // Auto-fit logic
        if (isIdle) {
            // "World Map" Zoom: Fit the world into the longest viewport dimension
            // 512 is the base world size at zoom 0 for MapLibre/Mapbox
            const maxDim = Math.max(ctx.mapWidth, ctx.mapHeight);
            newZoom = Math.log2(maxDim / 512);
        } else if (transition && simState === 'inactive') {
            newZoom = this.calculateAutoZoom(m, ctx, 1);
        } else {
            newZoom = this.calculateAutoZoom(m, ctx, 0);
        }

        this.lockedZoom = Math.round(newZoom * 2) / 2;
    }

    private calculateAutoZoom(m: maplibregl.Map, ctx: MapContext, _paddingMode: number): number {
        // Re-implement logic from original heartbeat if needed, 
        // for now we can default to holding current zoom or target base zoom
        // unless specific framing is requested.
        // Simplified for robust refactor start:
        return Math.max(ctx.zoom, m.getMinZoom());
    }

    private updateBearingLine(m: maplibregl.Map, acState: any, isReplay: boolean) {
        const lat1 = acState.lat * Math.PI / 180;
        const lon1 = acState.lon * Math.PI / 180;
        const brng = acState.heading * Math.PI / 180;
        const R = 3440.065; const d = 50.0;
        const lat2 = Math.asin(Math.sin(lat1) * Math.cos(d / R) + Math.cos(lat1) * Math.sin(d / R) * Math.cos(brng));
        const lon2 = lon1 + Math.atan2(Math.sin(brng) * Math.sin(d / R) * Math.cos(lat1), Math.cos(d / R) - Math.sin(lat1) * Math.sin(lat2));

        const bLine: Feature<any> = {
            type: 'Feature', properties: {},
            geometry: { type: 'LineString', coordinates: [[acState.lon, acState.lat], [lon2 * 180 / Math.PI, lat2 * 180 / Math.PI]] }
        };
        const lineSource = m.getSource('bearing-line') as maplibregl.GeoJSONSource;
        if (lineSource) lineSource.setData(bLine);
        if (m.getLayer('bearing-line-layer')) {
            m.setPaintProperty('bearing-line-layer', 'line-opacity', isReplay ? 0 : 0.7);
        }
    }
}
