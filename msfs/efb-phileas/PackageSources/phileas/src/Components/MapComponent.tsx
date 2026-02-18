import {
    ComponentProps, DisplayComponent, FSComponent, VNode, Subject,
    MapSystemBuilder, EventBus, Vec2Math, MapLayer,
    MapLayerProps, UnitType, GeoPoint, GNSSEvents
} from "@microsoft/msfs-sdk";

import "./MapComponent.scss";

interface MapComponentProps extends ComponentProps {
    bus: EventBus;
    telemetry: Subject<any>;
    pois: Subject<any[]>;
    settlements: Subject<any>;
    isVisible: Subject<boolean>;
}

/**
 * A custom layer to display Phileas POIs and Settlements.
 */
class PhileasPoiLayer extends MapLayer<MapLayerProps<any>> {
    private readonly containerRef = FSComponent.createRef<HTMLDivElement>();
    private pois: any[] = [];
    private subscriptions: any[] = [];
    private lastMarkerUpdate = 0;
    // BY DESIGN: Map overlay marker update frequency (5s) - maintained for performance/clutter control
    private readonly MARKER_UPDATE_INTERVAL = 5000;

    public onAttached(): void {
        const poisSub = (this.props.model as any).getModule("PhileasData").pois.sub((p: any[]) => {
            this.pois = p;
            this.updateMarkers(false);
        });
        this.subscriptions.push(poisSub);
    }

    public onMapProjectionChanged(): void {
        this.updateMarkers(true);
    }

    private updateMarkers(force: boolean): void {
        if (!this.containerRef.instance) return;

        const now = Date.now();
        if (!force && (now - this.lastMarkerUpdate < this.MARKER_UPDATE_INTERVAL)) {
            return;
        }
        this.lastMarkerUpdate = now;

        // Simple manual DOM management for performance (common in MSFS gauges)
        this.containerRef.instance.innerHTML = "";

        this.pois.forEach(poi => {
            const pos = new GeoPoint(poi.lat, poi.lon);
            const projected = this.props.mapProjection.project(pos, Vec2Math.create());
            const size = this.props.mapProjection.getProjectedSize();

            if (projected[0] >= 0 && projected[0] <= size[0] && projected[1] >= 0 && projected[1] <= size[1]) {
                const el = document.createElement("div");
                el.className = "poi-marker";
                el.style.position = "absolute";
                el.style.left = `${projected[0]}px`;
                el.style.top = `${projected[1]}px`;
                el.style.transform = "translate(-50%, -50%)";
                el.innerHTML = `<div class="poi-dot"></div><div class="poi-label">${poi.name}</div>`;
                this.containerRef.instance?.appendChild(el);
            }
        });
    }

    /** @inheritdoc */
    public render(): VNode {
        return (
            <div ref={this.containerRef} class="phileas-poi-layer" style="position: absolute; width: 100%; height: 100%; pointer-events: none;">
            </div>
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
            .withBing("efb-map")
            .withOwnAirplanePropBindings([], 1)
            .withRotation()
            .withOwnAirplaneIcon(32, "http://127.0.0.1:1920/icons/airfield.svg", Vec2Math.create(0.5, 0.5))
            .withModule("PhileasData", () => ({
                pois: this.props.pois,
                settlements: this.props.settlements
            }))
            .withLayer("PhileasPois", (context: any) => <PhileasPoiLayer model={context.model} mapProjection={context.projection} />, 100);

        this.mapSystem = builder.build("phileas-map-system");

        // Use live GNSS position from the sim bus for framing
        const gnss = this.props.bus.getSubscriber<GNSSEvents>();
        gnss.on('gps-position').handle((pos) => {
            this.planePos.set(pos.lat, pos.long);
            this.updateFraming(false);
        });
    }

    public onAfterRender(): void {
        this.updateSize();
        window.addEventListener('resize', () => this.updateSize());
        // BY DESIGN: Map resize check frequency (1s) - ensures map fills container correctly
        setInterval(() => this.updateSize(), 1000);
    }

    private updateFraming(force: boolean): void {
        const now = Date.now();
        if (!force && (now - this.lastFramingUpdate < this.FRAMING_INTERVAL)) {
            return;
        }
        this.lastFramingUpdate = now;

        if (!this.mapSystem) return;

        const projection = this.mapSystem.context.projection;
        const pois = this.props.pois.get() || [];

        // Compute bounding box around aircraft + POIs
        let minLat = this.planePos.lat;
        let maxLat = this.planePos.lat;
        let minLon = this.planePos.lon;
        let maxLon = this.planePos.lon;

        for (const p of pois) {
            if (p.lat !== undefined && p.lon !== undefined) {
                if (p.lat < minLat) minLat = p.lat;
                if (p.lat > maxLat) maxLat = p.lat;
                if (p.lon < minLon) minLon = p.lon;
                if (p.lon > maxLon) maxLon = p.lon;
            }
        }

        // 20% padding, minimum ~0.5nm worth of degrees
        const latSpan = maxLat - minLat;
        const lonSpan = maxLon - minLon;
        const latPad = Math.max(0.008, latSpan * 0.2);
        const lonPad = Math.max(0.008, lonSpan * 0.2);

        minLat -= latPad;
        maxLat += latPad;
        minLon -= lonPad;
        maxLon += lonPad;

        // Center the map on the bounding box midpoint
        const centerLat = (minLat + maxLat) / 2;
        const centerLon = (minLon + maxLon) / 2;
        const center = new GeoPoint(centerLat, centerLon);

        // Calculate range as the great-circle distance from center to a corner (in great-arc radians)
        const corner = new GeoPoint(maxLat, maxLon);
        const rangeRad = center.distance(corner);

        // Minimum range ~1 NM (in radians: 1nm / 3440.065nm per radian)
        const minRange = 1 / 3440.065;
        // Maximum range ~50 NM
        const maxRange = 50 / 3440.065;

        projection.set({
            target: center,
            range: Math.min(maxRange, Math.max(minRange, rangeRad)),
        });
    }

    private updateSize(): void {
        try {
            // Using mapSystem.ref (properly attached map VNode) for size measurements
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
                    // Force framing update when size changes
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
            <div class="map-system-container" style="width: 100%; height: 100%; position: relative;">
                {this.mapSystem.map}
            </div>
        );
    }
}
