import {
    ComponentProps, DisplayComponent, FSComponent, VNode, Subject,
    MapSystemBuilder, EventBus, Vec2Math, MapLayer,
    MapLayerProps, UnitType, GeoPoint
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

    constructor(props: MapComponentProps) {
        super(props);

        const builder = MapSystemBuilder.create(this.props.bus)
            .withProjectedSize(this.size)
            // BY DESIGN: Map system clock frequency (1Hz) - maintained for smooth transition/performance balance
            .withClockUpdate(1)
            .withBing("ebf-map")
            .withFollowAirplane()
            .withRotation()
            // BY DESIGN: Default range 5nm
            .withRange(UnitType.NMILE.createNumber(5))
            .withOwnAirplaneIcon(32, "http://127.0.0.1:1920/icons/airfield.svg", Vec2Math.create(0.5, 0.5))
            .withModule("PhileasData", () => ({
                pois: this.props.pois,
                settlements: this.props.settlements
            }))
            .withLayer("PhileasPois", (context: any) => <PhileasPoiLayer model={context.model} mapProjection={context.projection} />, 100);

        this.mapSystem = builder.build("phileas-map-system");

        // Force centering when telemetry becomes available
        this.props.telemetry.sub((t) => {
            if (t && t.Valid && this.mapSystem) {
                const pos = new GeoPoint(t.Latitude, t.Longitude);
                // Use the type-safe way to get the projection and cast if necessary
                const projection = this.mapSystem.projection as any;
                if (projection.setTarget) {
                    projection.setTarget(pos);
                }
            }
        }, true);
    }

    public onAfterRender(): void {
        this.updateSize();
        window.addEventListener('resize', () => this.updateSize());
        // BY DESIGN: Map resize check frequency (1s) - ensures map fills container correctly
        setInterval(() => this.updateSize(), 1000);
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
