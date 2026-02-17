import {
    ComponentProps, DisplayComponent, FSComponent, VNode, Subject,
    MapSystemBuilder, EventBus, MapSystemKeys, Vec2Math, MapLayer,
    MapProjection, MapLayerProps, MapSystemComponent, Subscribable,
    UnitType, NumberUnitInterface, UnitFamily, GeoPoint
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

    public onAttached(): void {
        console.log("PhileasPoiLayer: Attached");
        const poisSub = (this.props.model as any).getModule("PhileasData").pois.sub((p: any[]) => {
            this.pois = p;
            this.updateMarkers();
        });
        this.subscriptions.push(poisSub);
    }

    public onMapProjectionChanged(): void {
        this.updateMarkers();
    }

    private updateMarkers(): void {
        if (!this.containerRef.instance) return;

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
    private readonly mapRef = FSComponent.createRef<MapSystemComponent>();
    private readonly size = Subject.create(Vec2Math.create(100, 100));
    private mapSystem?: any;

    constructor(props: MapComponentProps) {
        super(props);
        console.log("MapComponent: Initializing stable map system");

        const builder = MapSystemBuilder.create(this.props.bus)
            .withProjectedSize(this.size)
            .withClockUpdate(10)
            .withBing("phileas-efb-map")
            .withFollowAirplane()
            .withRotation()
            .withOwnAirplaneIcon(32, "http://127.0.0.1:1920/icons/airfield.svg", Vec2Math.create(0.5, 0.5))
            .withModule("PhileasData", () => ({
                pois: this.props.pois,
                settlements: this.props.settlements
            }))
            .withLayer("PhileasPois", (context) => <PhileasPoiLayer model={context.model} mapProjection={context.projection} />, 100);

        this.mapSystem = builder.build("phileas-map-system");

        if (this.mapSystem.map) {
            (this.mapSystem.map as any).ref = this.mapRef;
            console.log("MapComponent: Ref attached to map VNode");
        }
    }

    public onAfterRender(): void {
        console.log("MapComponent: onAfterRender");
        this.updateSize();
        window.addEventListener('resize', () => this.updateSize());
        setInterval(() => this.updateSize(), 1000);
    }

    private updateSize(): void {
        try {
            const instance = this.mapRef.instance as any;
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
            // SDK throws 'Instance was null' if ref is accessed before mount
            // console.warn("MapComponent: updateSize deferred (instance not ready)");
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
