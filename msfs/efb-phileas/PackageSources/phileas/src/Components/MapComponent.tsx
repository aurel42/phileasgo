import {
    ComponentProps, DisplayComponent, FSComponent, VNode, Subject,
    MapSystemBuilder, EventBus, Vec2Math, MapLayer,
    MapLayerProps, GeoPoint,
    MapSystemKeys
} from "@microsoft/msfs-sdk";

import "./MapComponent.scss";

declare const BASE_URL: string;

const DISC_SIZE = 30;   // colored circle diameter (px)
const ICON_SIZE = 24;   // icon inside disc, 20% larger than original 20px
const COOLDOWN_MS = 8 * 60 * 60 * 1000; // matches repeat_ttl: 8h in phileas.yaml

// RGB packed as R | G<<8 | B<<16, required by MapTerrainColorsModule
function packColor(r: number, g: number, b: number): number {
    return r | (g << 8) | (b << 16);
}

// 61-entry earth color array (index 0 = water, 1-60 = terrain by elevation)
function buildEarthColors(): number[] {
    const colors: number[] = [];
    colors.push(packColor(3, 57, 108)); // water: deep ocean blue
    for (let i = 0; i < 60; i++) {
        const t = i / 59;
        let r: number, g: number, b: number;
        if (t < 0.25) {
            const s = t / 0.25;
            r = Math.round(46 + s * 34); g = Math.round(125 - s * 5); b = Math.round(50);
        } else if (t < 0.55) {
            const s = (t - 0.25) / 0.30;
            r = Math.round(80 + s * 40); g = Math.round(120 - s * 30); b = Math.round(50 - s * 10);
        } else if (t < 0.80) {
            const s = (t - 0.55) / 0.25;
            r = Math.round(120 + s * 30); g = Math.round(90 + s * 50); b = Math.round(40 + s * 90);
        } else {
            const s = (t - 0.80) / 0.20;
            r = Math.round(150 + s * 90); g = Math.round(140 + s * 100); b = Math.round(130 + s * 115);
        }
        colors.push(packColor(r, g, b));
    }
    return colors;
}

/** Resolve a POI icon name to its bundled asset URL, with fallback. */
function poiIconUrl(icon?: string): string {
    return `${BASE_URL}/assets/icons/${icon || 'attraction'}.svg`;
}

function lerpInt(a: number, b: number, t: number): number {
    return Math.round(a + (b - a) * Math.min(1, Math.max(0, t)));
}

/** Returns true if POI was played recently and is still within the 8h cooldown. */
function isOnCooldown(poi: any): boolean {
    const lp = poi.last_played;
    if (!lp) return false;
    const ts = new Date(lp).getTime();
    // NaN (unparseable), negative (year 0001 = Go zero time), or missing → not on cooldown
    if (isNaN(ts) || ts < 0) return false;
    return Date.now() - ts < COOLDOWN_MS;
}

/**
 * Disc color scheme (mirrors internal/ui dark map):
 *   playing   → bright green
 *   preparing → teal green
 *   cooldown  → blue
 *   otherwise → yellow (#E9C46A, score=0) → red (#e63946, score≥20)
 */
function poiDiscColor(poi: any, narratorStatus: any): string {
    if (narratorStatus) {
        if (poi.wikidata_id === narratorStatus.current_poi?.wikidata_id) return '#2ecc71';
        if (poi.wikidata_id === narratorStatus.preparing_poi?.wikidata_id) return '#2a9d8f';
    }
    if (isOnCooldown(poi)) return '#356285';

    const t = Math.min(20, Math.max(0, poi.score ?? 0)) / 20;
    return `#${lerpInt(0xe9, 0xe6, t).toString(16).padStart(2, '0')}` +
        `${lerpInt(0xc4, 0x39, t).toString(16).padStart(2, '0')}` +
        `${lerpInt(0x6a, 0x46, t).toString(16).padStart(2, '0')}`;
}

interface MapComponentProps extends ComponentProps {
    bus: EventBus;
    telemetry: Subject<any>;
    pois: Subject<any[]>;
    settlements: Subject<any>;
    isVisible: Subject<boolean>;
    narratorStatus: Subject<any>;
}

/** Cached DOM marker for a single POI. */
interface PoiMarker {
    id: string;
    wrapper: HTMLDivElement;
    img: HTMLImageElement;
    lat: number;
    lon: number;
}

/**
 * Custom layer rendering Phileas POIs as colored disc + white SVG icon markers.
 * DOM elements are created/removed only when the POI list changes; projection
 * changes only update positions — eliminating per-second flicker.
 */
class PhileasPoiLayer extends MapLayer<MapLayerProps<any>> {
    private readonly containerRef = FSComponent.createRef<HTMLDivElement>();
    private markers = new Map<string, PoiMarker>();
    private pois: any[] = [];
    private subscriptions: any[] = [];

    public onAttached(): void {
        const data = (this.props.model as any).getModule("PhileasData");

        this.subscriptions.push(data.pois.sub((p: any[]) => {
            this.pois = p;
            this.rebuildMarkers();
        }));

        // Recolor markers when narrator state changes (playing/preparing)
        this.subscriptions.push(data.narratorStatus.sub(() => {
            this.recolorMarkers();
        }));
    }

    public onMapProjectionChanged(): void {
        this.repositionMarkers();
    }

    /** Full rebuild: remove stale markers, create new ones, reposition all. */
    private rebuildMarkers(): void {
        if (!this.containerRef.instance) return;
        const container = this.containerRef.instance;
        const narratorStatus = (this.props.model as any).getModule("PhileasData").narratorStatus.get();

        // Determine which POI IDs are still present
        const currentIds = new Set<string>();
        for (const poi of this.pois) {
            if (poi.wikidata_id) currentIds.add(poi.wikidata_id);
        }

        // Remove markers for POIs no longer in the list
        for (const [id, marker] of this.markers) {
            if (!currentIds.has(id)) {
                marker.wrapper.remove();
                this.markers.delete(id);
            }
        }

        // Add or update markers
        for (const poi of this.pois) {
            const id = poi.wikidata_id;
            if (!id) continue;

            let marker = this.markers.get(id);
            if (!marker) {
                // Create new marker DOM
                const wrapper = document.createElement("div");
                const cooldown = isOnCooldown(poi);
                wrapper.style.cssText =
                    `position:absolute;width:${DISC_SIZE}px;height:${DISC_SIZE}px;` +
                    `transform:translate(-50%,-50%);border-radius:50%;pointer-events:none;` +
                    `background:${poiDiscColor(poi, narratorStatus)};` +
                    `border:1.5px solid rgba(0,0,0,0.45);` +
                    `box-shadow:0 2px 6px rgba(0,0,0,0.55);` +
                    `display:flex;align-items:center;justify-content:center;` +
                    `z-index:${cooldown ? 1 : 2};` +
                    `opacity:${cooldown ? '0.7' : '1'};`;

                const img = document.createElement("img");
                img.src = poiIconUrl(poi.icon);
                img.style.cssText = `width:${ICON_SIZE}px;height:${ICON_SIZE}px;` +
                    `filter:brightness(0) invert(1) ` +
                    `drop-shadow(0 1px 0 #000) drop-shadow(0 -1px 0 #000) ` +
                    `drop-shadow(1px 0 0 #000) drop-shadow(-1px 0 0 #000) ` +
                    `drop-shadow(0 0 4px rgba(255,255,255,0.75));`;

                wrapper.appendChild(img);
                container.appendChild(wrapper);
                marker = { id, wrapper, img, lat: poi.lat, lon: poi.lon };
                this.markers.set(id, marker);
            } else {
                // Update existing marker color and coords
                const cooldown = isOnCooldown(poi);
                marker.wrapper.style.background = poiDiscColor(poi, narratorStatus);
                marker.wrapper.style.zIndex = cooldown ? '1' : '2';
                marker.wrapper.style.opacity = cooldown ? '0.7' : '1';
                marker.lat = poi.lat;
                marker.lon = poi.lon;
            }
        }

        this.repositionMarkers();
    }

    /** Update only colors (narrator status changed). */
    private recolorMarkers(): void {
        const narratorStatus = (this.props.model as any).getModule("PhileasData").narratorStatus.get();
        for (const poi of this.pois) {
            const marker = this.markers.get(poi.wikidata_id);
            if (marker) {
                marker.wrapper.style.background = poiDiscColor(poi, narratorStatus);
            }
        }
    }

    /** Update only screen positions (projection changed). */
    private repositionMarkers(): void {
        const size = this.props.mapProjection.getProjectedSize();
        const cx = size[0] / 2;
        const cy = size[1] / 2;
        const targetProj = this.props.mapProjection.project(
            this.props.mapProjection.getTarget(), Vec2Math.create());

        for (const [, marker] of this.markers) {
            const projected = this.props.mapProjection.project(
                new GeoPoint(marker.lat, marker.lon), Vec2Math.create());
            const x = cx + (projected[0] - targetProj[0]);
            const y = cy + (projected[1] - targetProj[1]);

            if (x < -DISC_SIZE || x > size[0] + DISC_SIZE ||
                y < -DISC_SIZE || y > size[1] + DISC_SIZE) {
                marker.wrapper.style.display = 'none';
            } else {
                marker.wrapper.style.display = 'flex';
                marker.wrapper.style.left = `${x}px`;
                marker.wrapper.style.top = `${y}px`;
            }
        }
    }

    public render(): VNode {
        return (
            // top/left:0 ensures projected pixel coords map correctly to this layer
            <div ref={this.containerRef} class="phileas-poi-layer"
                style="position:absolute;top:0;left:0;width:100%;height:100%;pointer-events:none;" />
        );
    }

    public onDestroy(): void {
        this.subscriptions.forEach(s => s.destroy());
    }
}

/**
 * Custom airplane icon layer. Reads position/heading from the PhileasData
 * module (driven by HTTP telemetry) instead of the OwnAirplaneProps module
 * (which depends on SimVar bindings that don't exist in the EFB).
 */
class PhileasAirplaneLayer extends MapLayer<MapLayerProps<any>> {
    private readonly iconRef = FSComponent.createRef<HTMLDivElement>();

    public onMapProjectionChanged(): void {
        this.updateIcon();
    }

    public onUpdated(): void {
        this.updateIcon();
    }

    private updateIcon(): void {
        const el = this.iconRef.instance;
        if (!el) return;

        const data = (this.props.model as any).getModule("PhileasData");
        if (!data) return;

        const pos = data.planePosition.get();
        const heading = data.planeHeading.get();

        // Don't render until we have real telemetry
        if (!pos) {
            el.style.display = 'none';
            return;
        }

        const size = this.props.mapProjection.getProjectedSize();
        const cx = size[0] / 2;
        const cy = size[1] / 2;
        const targetProj = this.props.mapProjection.project(
            this.props.mapProjection.getTarget(), Vec2Math.create());
        const projected = this.props.mapProjection.project(
            new GeoPoint(pos.lat, pos.lon), Vec2Math.create());
        const x = cx + (projected[0] - targetProj[0]);
        const y = cy + (projected[1] - targetProj[1]);

        el.style.display = '';
        el.style.left = `${x}px`;
        el.style.top = `${y}px`;
        el.style.transform = `translate(-50%,-50%) rotate(${heading}deg)`;
    }

    public render(): VNode {
        return (
            <div style="position:absolute;top:0;left:0;width:100%;height:100%;pointer-events:none;">
                <div ref={this.iconRef}
                    style="position:absolute;width:32px;height:32px;pointer-events:none;display:none;">
                    <img src={`${BASE_URL}/assets/icons/airfield.svg`}
                        style="width:100%;height:100%;" />
                </div>
            </div>
        );
    }
}

export class MapComponent extends DisplayComponent<MapComponentProps> {
    private readonly size = Subject.create(Vec2Math.create(800, 800));
    private mapSystem?: any;

    // Plane state driven from HTTP telemetry, shared with layers via PhileasData module
    private readonly planePosition = Subject.create<{ lat: number, lon: number } | null>(null);
    private readonly planeHeading = Subject.create<number>(0);

    private planePos = new GeoPoint(0, 0);
    private lastFramingUpdate = 0;
    // BY DESIGN: Adaptive framing frequency matches main loop/map clock (1s)
    private readonly FRAMING_INTERVAL = 1000;

    constructor(props: MapComponentProps) {
        super(props);

        const builder = MapSystemBuilder.create(this.props.bus)
            .withProjectedSize(this.size)
            // BY DESIGN: Map system clock frequency (1Hz)
            .withClockUpdate(1)
            .withBing("bing")
            .withModule("PhileasData", () => ({
                pois: this.props.pois,
                settlements: this.props.settlements,
                narratorStatus: this.props.narratorStatus,
                planePosition: this.planePosition,
                planeHeading: this.planeHeading,
            }))
            .withLayer("PhileasPois", (context: any) =>
                <PhileasPoiLayer model={context.model} mapProjection={context.projection} />, 100)
            .withLayer("PhileasAirplane", (context: any) =>
                <PhileasAirplaneLayer model={context.model} mapProjection={context.projection} />, 200);

        this.mapSystem = builder.build("phileas-map-system");

        // Initialize Bing earth colors so the terrain layer renders instead of the black fallback
        const terrainModule = this.mapSystem.context.model.getModule(MapSystemKeys.TerrainColors);
        if (terrainModule) {
            terrainModule.colors.set(buildEarthColors());
            terrainModule.reference.set(0); // 0 = EBingReference.SEA
        }
    }

    public onAfterRender(): void {
        // Subscribe AFTER render — FSComponent does not deliver Subject
        // notifications to subscriptions created during the constructor.
        this.props.telemetry.sub((t) => {
            if (!t || !t.Valid) return;
            this.planePos.set(t.Latitude, t.Longitude);
            this.planePosition.set({ lat: t.Latitude, lon: t.Longitude });
            this.planeHeading.set(t.Heading);
            this.updateFraming(false);
        });

        this.props.pois.sub(() => this.updateFraming(false));

        this.updateSize();
        window.addEventListener('resize', () => this.updateSize());
        // BY DESIGN: Map resize check frequency (1s) - ensures map fills container correctly
        setInterval(() => this.updateSize(), 1000);

        // The EFB EventBus has no ClockPublisher, so `realTime` events never
        // fire and withClockUpdate(1) never triggers the MapSystem update cycle.
        // Drive it manually at 1 Hz so applyQueued() runs and layers update.
        setInterval(() => {
            try {
                this.mapSystem?.ref.instance?.update(Date.now());
            } catch {
                // map not yet ready or destroyed
            }
        }, 1000);
    }

    private updateFraming(force: boolean): void {
        const now = Date.now();
        if (!force && (now - this.lastFramingUpdate < this.FRAMING_INTERVAL)) return;
        this.lastFramingUpdate = now;

        if (!this.mapSystem) return;

        const projection = this.mapSystem.context.projection;
        const pois = this.props.pois.get() || [];

        let minLat = this.planePos.lat, maxLat = this.planePos.lat;
        let minLon = this.planePos.lon, maxLon = this.planePos.lon;

        for (const p of pois) {
            // Cooldown POIs (blue) are excluded from the framing bbox
            if (p.lat === undefined || p.lon === undefined || isOnCooldown(p)) continue;
            if (p.lat < minLat) minLat = p.lat;
            if (p.lat > maxLat) maxLat = p.lat;
            if (p.lon < minLon) minLon = p.lon;
            if (p.lon > maxLon) maxLon = p.lon;
        }

        const latSpan = maxLat - minLat;
        const lonSpan = maxLon - minLon;
        const latPad = Math.max(0.008, latSpan * 0.2);
        const lonPad = Math.max(0.008, lonSpan * 0.2);

        const centerLat = (minLat + maxLat) / 2;
        const centerLon = (minLon + maxLon) / 2;
        const rangeRad = new GeoPoint(centerLat, centerLon)
            .distance(new GeoPoint(maxLat + latPad, maxLon + lonPad));

        projection.setQueued({
            target: new GeoPoint(centerLat, centerLon),
            scaleFactor: null,
            range: Math.min(50 / 3440.065, Math.max(1 / 3440.065, rangeRad)),
        });
    }

    private updateSize(): void {
        try {
            const instance = this.mapSystem?.ref.instance as any;
            const container = instance?.parentElement;
            if (container) {
                const w = container.clientWidth;
                const h = container.clientHeight;
                if (w > 0 && h > 0) {
                    const current = this.size.get();
                    if (current[0] !== w || current[1] !== h) {
                        this.size.set(Vec2Math.create(w, h));
                    }
                    this.updateFraming(true);
                }
            }
        } catch (e) {
            // Silently fail to prevent console spam
        }
    }

    public render(): VNode {
        if (!this.mapSystem) return <div class="map-system-error">Map initialisation failed</div>;
        return (
            <div class="map-system-container" style="width:100%;height:100%;position:relative;">
                {this.mapSystem.map}
            </div>
        );
    }
}
