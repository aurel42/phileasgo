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

    public onAfterRender(): void {
        this.updateSize();
        window.addEventListener('resize', () => this.updateSize());

        // Poll for size changes as well (common in EFB environments)
        setInterval(() => this.updateSize(), 1000);
    }

    private updateSize(): void {
        const container = (this.mapRef.instance as any)?.parentElement;
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
    }

    public render(): VNode {
        // Initialize the MapSystem
        const builder = MapSystemBuilder.create(this.props.bus)
            .withProjectedSize(this.size)
            .withClockUpdate(10) // 10Hz for smoothness
            .withBing("phileas-efb-map")
            .withFollowAirplane()
            .withRotation()
            .withOwnAirplaneIcon(32, "coui://html_ui/Pages/VCockpit/Instruments/NavSystems/Shared/Images/Icons/Aircraft/airplane_icon.svg", Vec2Math.create(0.5, 0.5))
            .withModule("PhileasData", () => ({
                pois: this.props.pois,
                settlements: this.props.settlements
            }))
            .withLayer("PhileasPois", (context) => <PhileasPoiLayer model={context.model} mapProjection={context.projection} />, 100);

        const compiled = builder.build("phileas-map-system");

        return (
            <div class="map-system-container" style="width: 100%; height: 100%; position: relative;">
                {compiled.map}
            </div>
        );
    }
}
