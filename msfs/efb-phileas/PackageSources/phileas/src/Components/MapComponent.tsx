import {
    ComponentProps, DisplayComponent, FSComponent, VNode, Subject,
    MapSystemBuilder, EventBus, Vec2Math, MapLayer,
    MapLayerProps, GeoPoint, GNSSEvents,
    MapSystemKeys, MapOwnAirplanePropsKey, EBingReference
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
    if (!poi.last_played || poi.last_played === '0001-01-01T00:00:00Z') return false;
    return Date.now() - new Date(poi.last_played).getTime() < COOLDOWN_MS;
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
        if (poi.wikidata_id === narratorStatus.current_poi?.wikidata_id)   return '#2ecc71';
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

/**
 * Custom layer rendering Phileas POIs as colored disc + white SVG icon markers.
 */
class PhileasPoiLayer extends MapLayer<MapLayerProps<any>> {
    private readonly containerRef = FSComponent.createRef<HTMLDivElement>();
    private pois: any[] = [];
    private subscriptions: any[] = [];
    private lastMarkerUpdate = 0;
    // BY DESIGN: POI marker update frequency (5s) - maintained for performance/clutter control
    private readonly MARKER_UPDATE_INTERVAL = 5000;

    public onAttached(): void {
        const data = (this.props.model as any).getModule("PhileasData");

        this.subscriptions.push(data.pois.sub((p: any[]) => {
            this.pois = p;
            this.updateMarkers(false);
        }));

        // Recolor markers when narrator state changes (playing/preparing)
        this.subscriptions.push(data.narratorStatus.sub(() => {
            this.updateMarkers(true);
        }));
    }

    public onMapProjectionChanged(): void {
        this.updateMarkers(true);
    }

    private updateMarkers(force: boolean): void {
        if (!this.containerRef.instance) return;

        const now = Date.now();
        if (!force && (now - this.lastMarkerUpdate < this.MARKER_UPDATE_INTERVAL)) return;
        this.lastMarkerUpdate = now;

        const container = this.containerRef.instance;
        container.innerHTML = "";

        const narratorStatus = (this.props.model as any).getModule("PhileasData").narratorStatus.get();

        for (const poi of this.pois) {
            const projected = this.props.mapProjection.project(new GeoPoint(poi.lat, poi.lon), Vec2Math.create());
            const size = this.props.mapProjection.getProjectedSize();
            if (projected[0] < 0 || projected[0] > size[0] || projected[1] < 0 || projected[1] > size[1]) continue;

            const wrapper = document.createElement("div");
            wrapper.style.cssText = `position:absolute;left:${projected[0]}px;top:${projected[1]}px;` +
                `transform:translate(-50%,-50%);width:${DISC_SIZE}px;height:${DISC_SIZE}px;` +
                `border-radius:50%;pointer-events:none;` +
                `background:${poiDiscColor(poi, narratorStatus)};` +
                `border:1.5px solid rgba(0,0,0,0.45);` +
                `box-shadow:0 2px 6px rgba(0,0,0,0.55);` +
                `display:flex;align-items:center;justify-content:center;`;

            const img = document.createElement("img");
            img.src = poiIconUrl(poi.icon);
            img.style.cssText = `width:${ICON_SIZE}px;height:${ICON_SIZE}px;` +
                // Make icon white, add 1px black outline (4 directions) + white halo
                `filter:brightness(0) invert(1) ` +
                `drop-shadow(0 1px 0 #000) drop-shadow(0 -1px 0 #000) ` +
                `drop-shadow(1px 0 0 #000) drop-shadow(-1px 0 0 #000) ` +
                `drop-shadow(0 0 4px rgba(255,255,255,0.75));`;

            wrapper.appendChild(img);
            container.appendChild(wrapper);
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

export class MapComponent extends DisplayComponent<MapComponentProps> {
    private readonly size = Subject.create(Vec2Math.create(800, 800));
    private mapSystem?: any;

    private planePos = new GeoPoint(0, 0);
    private lastFramingUpdate = 0;
    // BY DESIGN: Adaptive framing frequency matches main loop/map clock (1s)
    private readonly FRAMING_INTERVAL = 1000;

    constructor(props: MapComponentProps) {
        super(props);

        const builder = MapSystemBuilder.create(this.props.bus)
            .withProjectedSize(this.size)
            // BY DESIGN: Map system clock frequency (1Hz) - maintained for smooth transition/performance balance
            .withClockUpdate(1)
            .withBing("bing")
            // FIXED: 'position' binding required for the own-airplane icon to track aircraft location
            .withOwnAirplanePropBindings([MapOwnAirplanePropsKey.Position], 1)
            .withRotation()
            .withOwnAirplaneIcon(32, `${BASE_URL}/assets/icons/airfield.svg`, Vec2Math.create(0.5, 0.5))
            .withModule("PhileasData", () => ({
                pois: this.props.pois,
                settlements: this.props.settlements,
                narratorStatus: this.props.narratorStatus,
            }))
            .withLayer("PhileasPois", (context: any) =>
                <PhileasPoiLayer model={context.model} mapProjection={context.projection} />, 100);

        this.mapSystem = builder.build("phileas-map-system");

        // Initialize Bing earth colors so the terrain layer renders instead of the black fallback
        const terrainModule = this.mapSystem.context.model.getModule(MapSystemKeys.TerrainColors);
        if (terrainModule) {
            terrainModule.colors.set(buildEarthColors());
            terrainModule.reference.set(EBingReference.SEA);
        }

        const gnss = this.props.bus.getSubscriber<GNSSEvents>();
        gnss.on('gps-position').handle((pos) => {
            this.planePos.set(pos.lat, pos.long);
            this.updateFraming(false);
        });

        // Reframe when POIs change (e.g. plane stationary but new POIs load)
        this.props.pois.sub(() => this.updateFraming(false));
    }

    public onAfterRender(): void {
        this.updateSize();
        window.addEventListener('resize', () => this.updateSize());
        // BY DESIGN: Map resize check frequency (1s) - ensures map fills container correctly
        setInterval(() => this.updateSize(), 1000);
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
            // Issue 4: cooldown POIs (blue) are excluded from the framing bbox
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

        projection.set({
            target: new GeoPoint(centerLat, centerLon),
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
