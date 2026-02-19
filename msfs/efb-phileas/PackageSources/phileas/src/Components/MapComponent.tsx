import {
    ComponentProps, DisplayComponent, FSComponent, VNode, Subject,
    MapSystemBuilder, EventBus, Vec2Math, MapLayer,
    MapLayerProps, GeoPoint, MapProjection,
    MapSystemKeys
} from "@microsoft/msfs-sdk";

import { aircraftSvgPaths, AircraftType } from "./AircraftIcon";

import "./MapComponent.scss";

declare const BASE_URL: string;

const DISC_SIZE = 30;   // colored circle diameter (px)
const ICON_SIZE = 24;   // icon inside disc, 20% larger than original 20px
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

/** Returns true if POI was played recently and is still within the cooldown. */
function isOnCooldown(poi: any): boolean {
    return !!poi.is_on_cooldown;
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

function poiScaleFactor(poi: any, narratorStatus: any): number {
    if (narratorStatus) {
        if (poi.wikidata_id === narratorStatus.current_poi?.wikidata_id) return 1.5;
        if (poi.wikidata_id === narratorStatus.preparing_poi?.wikidata_id) return 1.25;
    }
    return 1.0;
}

interface MapComponentProps extends ComponentProps {
    bus: EventBus;
    telemetry: Subject<any>;
    pois: Subject<any[]>;
    settlements: Subject<any>;
    isVisible: Subject<boolean>;
    narratorStatus: Subject<any>;
    aircraftConfig: Subject<any>;
}

/** Cached DOM marker for a single POI. */
interface PoiMarker {
    id: string;
    wrapper: HTMLDivElement;
    img: HTMLImageElement;
    beacon: HTMLDivElement | undefined;
    beaconColor: string | undefined;
    lat: number;
    lon: number;
    scale: number;
}

/** Creates a beacon pin element (plain DOM — no FSComponent lifecycle). */
function createBeaconElement(color: string, size: number): HTMLDivElement {
    const wrapper = document.createElement('div');
    wrapper.style.cssText = 'position:absolute;transform:translate(-50%,-100%);pointer-events:none;';

    const haloSize = size * 2;
    const halo = document.createElement('div');
    halo.style.cssText = `position:absolute;left:50%;top:50%;width:${haloSize}px;height:${haloSize}px;` +
        `background:radial-gradient(circle,${color}66 0%,transparent 70%);` +
        `transform:translate(-50%,-50%);border-radius:50%;pointer-events:none;`;
    wrapper.appendChild(halo);

    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('width', String(size));
    svg.setAttribute('height', String(size * 1.25));
    svg.setAttribute('viewBox', '0 0 40 50');
    svg.setAttribute('preserveAspectRatio', 'xMidYMid meet');
    svg.style.cssText = `position:relative;display:block;width:${size}px;height:${size * 1.25}px;pointer-events:none;filter:drop-shadow(0 0 1px white);`;
    svg.innerHTML =
        `<path d="M20,5 C12,5 5,12 5,22 C5,28 10,35 20,42 C30,35 35,28 35,22 C35,12 28,5 20,5" ` +
        `fill="${color}" stroke="black" stroke-width="1.5"/>` +
        `<line x1="12" y1="36" x2="16" y2="42" stroke="black" stroke-width="1" />` +
        `<line x1="28" y1="36" x2="24" y2="42" stroke="black" stroke-width="1" />` +
        `<rect x="16" y="42" width="8" height="6" rx="1" fill="#1a1a1a" stroke="black" stroke-width="1" />`;
    wrapper.appendChild(svg);

    return wrapper;
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
    private data: any;

    // Scratch objects — reused every projection cycle to avoid per-frame allocations
    private readonly targetVec = Vec2Math.create();
    private readonly projVec = Vec2Math.create();
    private readonly geoScratch = new GeoPoint(0, 0);

    public onAttached(): void {
        this.data = (this.props.model as any).getModule("PhileasData");

        this.subscriptions.push(this.data.pois.sub((p: any[]) => {
            this.pois = p;
            this.rebuildMarkers();
        }, true));

        // Recolor markers when narrator state changes (playing/preparing)
        this.subscriptions.push(this.data.narratorStatus.sub(() => {
            this.recolorMarkers();
        }, true));
    }

    public onMapProjectionChanged(_mapProjection: MapProjection, _changeFlags: number): void {
        this.repositionMarkers();
    }

    /** Full rebuild: remove stale markers, create new ones, reposition all. */
    private rebuildMarkers(): void {
        if (!this.containerRef.instance) return;
        const container = this.containerRef.instance;
        const narratorStatus = this.data.narratorStatus.get();

        // Determine which POI IDs are still present
        const currentIds = new Set<string>();
        for (const poi of this.pois) {
            if (poi.wikidata_id) currentIds.add(poi.wikidata_id);
        }

        // Remove markers for POIs no longer in the list
        for (const [id, marker] of this.markers) {
            if (!currentIds.has(id)) {
                marker.wrapper.remove();
                if (marker.beacon) marker.beacon.remove();
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

                const scale = poiScaleFactor(poi, narratorStatus);
                wrapper.style.transform = `translate(-50%,-50%) scale(${scale})`;

                marker = { id, wrapper, img, beacon: undefined, beaconColor: undefined, lat: poi.lat, lon: poi.lon, scale };

                if (poi.beacon_color) {
                    marker.beacon = createBeaconElement(poi.beacon_color, DISC_SIZE * 0.5);
                    marker.beacon.style.zIndex = '3';
                    marker.beaconColor = poi.beacon_color;
                    container.appendChild(marker.beacon);
                }

                this.markers.set(id, marker);
            } else {
                // Update existing marker color and coords
                const cooldown = isOnCooldown(poi);
                const scale = poiScaleFactor(poi, narratorStatus);
                marker.wrapper.style.background = poiDiscColor(poi, narratorStatus);
                marker.wrapper.style.zIndex = cooldown ? '1' : '2';
                marker.wrapper.style.opacity = cooldown ? '0.7' : '1';
                marker.wrapper.style.transform = `translate(-50%,-50%) scale(${scale})`;
                marker.scale = scale;
                marker.lat = poi.lat;
                marker.lon = poi.lon;

                // Update beacon: create, remove, or rebuild if color changed
                if (poi.beacon_color) {
                    if (!marker.beacon || marker.beaconColor !== poi.beacon_color) {
                        if (marker.beacon) marker.beacon.remove();
                        marker.beacon = createBeaconElement(poi.beacon_color, DISC_SIZE * 0.5);
                        marker.beacon.style.zIndex = '3';
                        marker.beaconColor = poi.beacon_color;
                        container.appendChild(marker.beacon);
                    }
                } else if (marker.beacon) {
                    marker.beacon.remove();
                    marker.beacon = undefined;
                    marker.beaconColor = undefined;
                }
            }
        }

        this.repositionMarkers();
    }

    /** Update only colors (narrator status changed). */
    private recolorMarkers(): void {
        const narratorStatus = this.data.narratorStatus.get();
        const container = this.containerRef.instance;
        if (!container) return;

        let needsReposition = false;
        for (const poi of this.pois) {
            const marker = this.markers.get(poi.wikidata_id);
            if (!marker) continue;

            marker.wrapper.style.background = poiDiscColor(poi, narratorStatus);
            const scale = poiScaleFactor(poi, narratorStatus);
            marker.wrapper.style.transform = `translate(-50%,-50%) scale(${scale})`;
            marker.scale = scale;

            if (poi.beacon_color) {
                if (!marker.beacon || marker.beaconColor !== poi.beacon_color) {
                    if (marker.beacon) marker.beacon.remove();
                    marker.beacon = createBeaconElement(poi.beacon_color, DISC_SIZE * 0.5);
                    marker.beacon.style.zIndex = '3';
                    marker.beaconColor = poi.beacon_color;
                    container.appendChild(marker.beacon);
                    needsReposition = true;
                }
            } else if (marker.beacon) {
                marker.beacon.remove();
                marker.beacon = undefined;
                marker.beaconColor = undefined;
            }
        }
        if (needsReposition) this.repositionMarkers();
    }

    /** Update only screen positions (projection changed). */
    private repositionMarkers(): void {
        const size = this.props.mapProjection.getProjectedSize();
        const cx = size[0] / 2;
        const cy = size[1] / 2;
        this.props.mapProjection.project(
            this.props.mapProjection.getTarget(), this.targetVec);

        for (const [, marker] of this.markers) {
            this.geoScratch.set(marker.lat, marker.lon);
            this.props.mapProjection.project(this.geoScratch, this.projVec);
            const x = cx + (this.projVec[0] - this.targetVec[0]);
            const y = cy + (this.projVec[1] - this.targetVec[1]);

            if (x < -DISC_SIZE || x > size[0] + DISC_SIZE ||
                y < -DISC_SIZE || y > size[1] + DISC_SIZE) {
                marker.wrapper.style.display = 'none';
                if (marker.beacon) marker.beacon.style.display = 'none';
            } else {
                marker.wrapper.style.display = 'flex';
                marker.wrapper.style.left = `${x}px`;
                marker.wrapper.style.top = `${y}px`;

                if (marker.beacon) {
                    marker.beacon.style.display = 'block';
                    marker.beacon.style.left = `${x}px`;
                    marker.beacon.style.top = `${y - (DISC_SIZE * marker.scale) / 2 - 2}px`;
                }
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
 *
 * Uses imperative DOM — render() creates an empty wrapper; onAttached()
 * subscribes to aircraftConfig and rebuilds the SVGs when the icon/colors change.
 * updateIcon() runs per tick to set position, rotation, and shadow offset.
 */
class PhileasAirplaneLayer extends MapLayer<MapLayerProps<any>> {
    private readonly iconRef = FSComponent.createRef<HTMLDivElement>();
    private shadowSvg: SVGSVGElement | null = null;
    private currentConfig: any = null;
    private subscriptions: any[] = [];
    private data: any;

    // Scratch objects — reused every tick to avoid per-frame allocations
    private readonly targetVec = Vec2Math.create();
    private readonly projVec = Vec2Math.create();
    private readonly geoScratch = new GeoPoint(0, 0);

    public onAttached(): void {
        this.data = (this.props.model as any).getModule("PhileasData");

        this.subscriptions.push(this.data.aircraftConfig.sub((config: any) => {
            if (!config) return;
            this.currentConfig = config;
            this.rebuildIcon();
        }, true));
    }

    /** Rebuild main + shadow SVGs from current config. */
    private rebuildIcon(): void {
        const el = this.iconRef.instance;
        if (!el || !this.currentConfig) return;

        const config = this.currentConfig;
        const iconType = (config.aircraft_icon || 'jet') as AircraftType;
        const size = config.aircraft_size || 32;
        const colorMain = config.aircraft_color_main || '#e67e22';
        const colorAccent = config.aircraft_color_accent || '#ffffff';

        // Clear previous content
        el.innerHTML = '';
        this.shadowSvg = null;

        // Shadow SVG — all paths filled black
        // Position centered; per-tick transform handles AGL offset + scale (GPU-composited)
        const shadow = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
        shadow.setAttribute('viewBox', '0 0 100 100');
        shadow.style.cssText =
            `position:absolute;left:50%;top:50%;width:${size}px;height:${size}px;` +
            `transform-origin:center;transform:translate(-50%,-50%);` +
            `filter:blur(3px);opacity:0.3;`;
        shadow.innerHTML = aircraftSvgPaths(iconType, 'black', 'black');
        el.appendChild(shadow);
        this.shadowSvg = shadow;

        // Main icon SVG
        const main = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
        main.setAttribute('viewBox', '0 0 100 100');
        main.style.cssText =
            `position:absolute;left:50%;top:50%;width:${size}px;height:${size}px;` +
            `transform:translate(-50%,-50%);filter:drop-shadow(0px 2px 2px rgba(0,0,0,0.3));`;
        main.innerHTML = aircraftSvgPaths(iconType, colorMain, colorAccent);
        el.appendChild(main);
    }

    public onUpdated(_time: number, _elapsed: number): void {
        this.updateIcon();
    }

    private updateIcon(): void {
        const el = this.iconRef.instance;
        if (!el) return;

        const pos = this.data.planePosition.get();
        const heading = this.data.planeHeading.get();
        const tel = this.data.telemetry.get();
        const config = this.currentConfig;

        if (!pos || !tel || !config) {
            el.style.display = 'none';
            return;
        }

        const projSize = this.props.mapProjection.getProjectedSize();
        const cx = projSize[0] / 2;
        const cy = projSize[1] / 2;
        this.props.mapProjection.project(
            this.props.mapProjection.getTarget(), this.targetVec);
        this.geoScratch.set(pos.lat, pos.lon);
        this.props.mapProjection.project(this.geoScratch, this.projVec);
        const x = cx + (this.projVec[0] - this.targetVec[0]);
        const y = cy + (this.projVec[1] - this.targetVec[1]);

        el.style.display = '';
        el.style.left = `${x}px`;
        el.style.top = `${y}px`;

        // Balloons stay upright
        if (config.aircraft_icon !== 'balloon') {
            el.style.transform = `translate(-50%,-50%) rotate(${heading}deg)`;
        } else {
            el.style.transform = `translate(-50%,-50%)`;
        }

        // Update shadow offset/scale based on altitude AGL (transform-only, GPU-composited)
        if (this.shadowSvg) {
            const agl = tel.AltitudeAGL ?? 0;
            const ratio = Math.min(Math.max(agl / 10000, 0), 1);
            const size = config.aircraft_size || 32;
            const offset = ratio * (size * 0.6);
            const scale = 1 - (ratio * 0.5);
            this.shadowSvg.style.transform =
                `translate(calc(-50% + ${offset}px), calc(-50% + ${offset}px)) scale(${scale})`;
        }
    }

    public render(): VNode {
        return (
            <div style="position:absolute;top:0;left:0;width:100%;height:100%;pointer-events:none;">
                <div ref={this.iconRef}
                    style="position:absolute;pointer-events:none;display:none;" />
            </div>
        );
    }

    public onDestroy(): void {
        this.subscriptions.forEach(s => s.destroy());
    }
}

export class MapComponent extends DisplayComponent<MapComponentProps> {
    private readonly size = Subject.create(Vec2Math.create(400, 400));
    private readonly containerRef = FSComponent.createRef<HTMLDivElement>();
    private mapSystem?: any;

    // Plane state driven from HTTP telemetry, shared with layers via PhileasData module
    private readonly planePosition = Subject.create<{ lat: number, lon: number } | null>(null);
    private readonly planeHeading = Subject.create<number>(0);

    private lastFramingUpdate = 0;
    // BY DESIGN: Framing frequency (5s) — avoids micro-zoom adjustments every tick
    private readonly FRAMING_INTERVAL = 5000;

    // Scratch GeoPoints for updateFraming — reused to avoid per-second allocations
    private readonly framingScratchA = new GeoPoint(0, 0);
    private readonly framingScratchB = new GeoPoint(0, 0);

    private subscriptions: any[] = [];
    private intervalHandles: number[] = [];
    private readonly resizeHandler = () => this.updateSize();

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
                telemetry: this.props.telemetry,
                aircraftConfig: this.props.aircraftConfig,
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

    public onAfterRender(node: VNode): void {
        super.onAfterRender(node);
        // Subscribe AFTER render — FSComponent does not deliver Subject
        // notifications to subscriptions created during the constructor.
        this.subscriptions.push(this.props.telemetry.sub((t) => {
            if (!t || !t.Valid) return;
            this.planePosition.set({ lat: t.Latitude, lon: t.Longitude });
            this.planeHeading.set(t.Heading);
            this.updateFraming(false);
        }));

        this.subscriptions.push(this.props.pois.sub(() => this.updateFraming(false)));

        this.updateSize();
        window.addEventListener('resize', this.resizeHandler);
        // BY DESIGN: Map resize check frequency (1s) - ensures map fills container correctly
        this.intervalHandles.push(window.setInterval(() => this.updateSize(), 1000));

        // The EFB EventBus has no ClockPublisher, so `realTime` events never
        // fire and withClockUpdate(1) never triggers the MapSystem update cycle.
        // Drive it manually at 1 Hz so applyQueued() runs and layers update.
        this.intervalHandles.push(window.setInterval(() => {
            try {
                this.mapSystem?.ref.instance?.update(Date.now());
            } catch {
                // map not yet ready or destroyed
            }
        }, 1000));
    }

    private updateFraming(force: boolean): void {
        const now = Date.now();
        if (!force && (now - this.lastFramingUpdate < this.FRAMING_INTERVAL)) return;
        this.lastFramingUpdate = now;

        if (!this.mapSystem) return;

        const tel = this.props.telemetry.get();
        if (!tel || !tel.Valid) return;

        const pois = this.props.pois.get() || [];
        const narrator = this.props.narratorStatus.get();
        const playingId = narrator?.current_poi?.id || narrator?.current_poi?.wikidata_id;
        const preparingId = narrator?.preparing_poi?.id || narrator?.preparing_poi?.wikidata_id;

        // Always center on aircraft position
        const acLat = tel.Latitude;
        const acLon = tel.Longitude;

        // Selection A: Non-cooldown POIs + playing/preparing — track max offset from aircraft
        let maxLatDiff = 0, maxLonDiff = 0;
        let hasSelection = false;

        for (const p of pois) {
            if (p.lat === undefined || p.lon === undefined) continue;
            const isCooldown = isOnCooldown(p);
            const isPlaying = playingId && (p.id === playingId || p.wikidata_id === playingId);
            const isPreparing = preparingId && (p.id === preparingId || p.wikidata_id === preparingId);
            if (isCooldown && !isPlaying && !isPreparing) continue;
            maxLatDiff = Math.max(maxLatDiff, Math.abs(p.lat - acLat));
            maxLonDiff = Math.max(maxLonDiff, Math.abs(p.lon - acLon));
            hasSelection = true;
        }

        // Selection B Fallback: if A yielded nothing, expand over ALL POIs
        if (!hasSelection) {
            for (const p of pois) {
                if (p.lat === undefined || p.lon === undefined) continue;
                maxLatDiff = Math.max(maxLatDiff, Math.abs(p.lat - acLat));
                maxLonDiff = Math.max(maxLonDiff, Math.abs(p.lon - acLon));
            }
        }

        let range: number;

        if (maxLatDiff === 0 && maxLonDiff === 0) {
            // Aircraft only — no POIs or all co-located
            const isOnGround = !!(tel.OnGround);
            // 4km on ground (~2.16nm), 50km in air (~27nm)
            range = (isOnGround ? 2.16 : 27) / 3440.065;
        } else {
            // Padding: buffer for POI disc clearance
            const latPad = Math.max(0.005, maxLatDiff * 0.15);
            const lonPad = Math.max(0.005, maxLonDiff * 0.15);
            const paddedLatDiff = maxLatDiff + latPad;
            const paddedLonDiff = maxLonDiff + lonPad;

            // range = full viewport height (default rangeEndpoints [0.5,0,0.5,1])
            // Vertical: full extent from aircraft ± paddedLatDiff
            this.framingScratchA.set(acLat - paddedLatDiff, acLon);
            this.framingScratchB.set(acLat + paddedLatDiff, acLon);
            const vertRange = this.framingScratchA.distance(this.framingScratchB);

            // Horizontal: full extent, then aspect-ratio-correct to equivalent range
            this.framingScratchA.set(acLat, acLon - paddedLonDiff);
            this.framingScratchB.set(acLat, acLon + paddedLonDiff);
            const horizExtent = this.framingScratchA.distance(this.framingScratchB);
            const projSize = this.mapSystem.context.projection.getProjectedSize();
            const horizRange = horizExtent * (projSize[1] / projSize[0]);

            range = Math.max(vertRange, horizRange);

            // Clamp max range to 60nm
            range = Math.min(60 / 3440.065, range);
        }

        this.mapSystem.context.projection.setQueued({
            target: this.framingScratchA.set(acLat, acLon),
            scaleFactor: null,
            range: Math.max(1.5 / 3440.065, range) // min 1.5nm for visibility
        });
    }

    private updateSize(): void {
        const container = this.containerRef.instance;
        if (!container) return;
        const w = container.clientWidth;
        const h = container.clientHeight;
        if (w > 0 && h > 0) {
            const current = this.size.get();
            if (current[0] !== w || current[1] !== h) {
                this.size.set(Vec2Math.create(w, h));
                this.updateFraming(true);
            }
        }
    }

    public render(): VNode {
        if (!this.mapSystem) return <div class="map-system-error">Map initialisation failed</div>;
        return (
            <div ref={this.containerRef} class="map-system-container" style="width:100%;height:100%;position:relative;">
                {this.mapSystem.map}
            </div>
        );
    }

    public destroy(): void {
        this.subscriptions.forEach(s => s.destroy());
        this.intervalHandles.forEach(h => window.clearInterval(h));
        window.removeEventListener('resize', this.resizeHandler);
        super.destroy();
    }
}
