import { TTButton, GamepadUiView, RequiredProps, TVNode, UiViewProps, List } from "@efb/efb-api";
import { FSComponent, Subject, VNode, ArraySubject, EventBus } from "@microsoft/msfs-sdk";
import { Logger } from "../Utils/Logger";
import { MapComponent } from "./MapComponent";

import "./PhileasPage.scss";

declare const VERSION: string;
declare const BUILD_TIMESTAMP: string;

interface PhileasPageProps extends RequiredProps<UiViewProps, "appViewService"> {
    bus: EventBus;
    telemetry: Subject<any>;
    pois: Subject<any[]>;
    settlements: Subject<any>;
}

interface TelemetryItem {
    label: string;
    value: string;
}

interface PoiItem {
    name: string;
    city: string;
    score: number;
    distance: number;
}

interface SettlementItem {
    name: string;
    pop: number;
    distance: number;
}

export class PhileasPage extends GamepadUiView<HTMLDivElement, PhileasPageProps> {
    public readonly tabName = PhileasPage.name;
    public declare readonly props: PhileasPageProps;

    private readonly activeTab = Subject.create<string>("map");

    // Throttled Subjects for UI Binding using ArraySubject for List compatibility
    private readonly uiTelemetry = ArraySubject.create<TelemetryItem>([]);
    private readonly uiPois = ArraySubject.create<PoiItem>([]);
    private readonly uiSettlements = ArraySubject.create<SettlementItem>([]);

    private readonly isMapVisible = Subject.create<boolean>(true);

    private lastUpdate = 0;
    private readonly updateInterval = 5000;
    private subscriptions: any[] = [];

    // Refs for visibility control
    private readonly mapContainerRef = FSComponent.createRef<HTMLDivElement>();
    private readonly dashboardContainerRef = FSComponent.createRef<HTMLDivElement>();
    private readonly poisContainerRef = FSComponent.createRef<HTMLDivElement>();
    private readonly settlementsContainerRef = FSComponent.createRef<HTMLDivElement>();

    // Helper for formatted telemetry display
    private readonly telemDisplay = {
        lat: Subject.create<string>("-"),
        lon: Subject.create<string>("-"),
        alt: Subject.create<string>("-"),
        hdg: Subject.create<string>("-")
    };

    public onAfterRender(): void {
        // Tab switching logic
        this.activeTab.sub(tab => {
            const isMap = tab === 'map';
            if (this.isMapVisible.get() !== isMap) {
                this.isMapVisible.set(isMap);
            }

            // Manual style updates because FSComponent doesn't reactive-bind inline style objects
            if (this.mapContainerRef.instance) this.mapContainerRef.instance.style.display = tab === 'map' ? 'block' : 'none';
            if (this.dashboardContainerRef.instance) this.dashboardContainerRef.instance.style.display = tab === 'dashboard' ? 'block' : 'none';
            if (this.poisContainerRef.instance) this.poisContainerRef.instance.style.display = tab === 'pois' ? 'block' : 'none';
            if (this.settlementsContainerRef.instance) this.settlementsContainerRef.instance.style.display = tab === 'settlements' ? 'block' : 'none';

            this.updateUiData(true);
        });

        // Subscribe to raw data props
        const telemSub = this.props.telemetry.sub(() => this.updateUiData(false));
        const poisSub = this.props.pois.sub(() => this.updateUiData(false));
        const settleSub = this.props.settlements.sub(() => this.updateUiData(false));

        this.subscriptions.push(telemSub, poisSub, settleSub);

        // Initial update
        this.updateUiData(true);
    }

    private updateUiData(force: boolean) {
        const now = Date.now();
        if (!force && (now - this.lastUpdate < this.updateInterval)) {
            return;
        }

        this.lastUpdate = now;
        console.log("PhileasPage: updateUiData triggered");

        // 1. Update Telemetry Display (for Dashboard) & List
        const t = this.props.telemetry.get();
        if (t) {
            this.telemDisplay.lat.set(t.Latitude?.toFixed(4) ?? "-");
            this.telemDisplay.lon.set(t.Longitude?.toFixed(4) ?? "-");
            this.telemDisplay.alt.set(t.Altitude?.toFixed(0) ?? "-");
            this.telemDisplay.hdg.set(t.Heading?.toFixed(0) ?? "-");

            this.uiTelemetry.set([
                { label: "Speed", value: `${(t.Speed || 0).toFixed(0)} kts` },
                { label: "Altitude", value: `${(t.Altitude || 0).toFixed(0)} ft` },
                { label: "Heading", value: `${(t.Heading || 0).toFixed(0)}°` },
                { label: "Lat/Lon", value: `${t.Latitude?.toFixed(4)} / ${t.Longitude?.toFixed(4)}` }
            ]);
        } else {
            console.log("PhileasPage: Telemetry is null/undefined");
        }

        // 2. Update POIs
        const rawPois = this.props.pois.get() || [];
        console.log(`PhileasPage: Raw POIs count: ${rawPois.length}`);
        const sortedPois = [...rawPois].sort((a: any, b: any) => (b.score || 0) - (a.score || 0)).slice(0, 50);
        this.uiPois.set(sortedPois.map((p: any) => ({
            name: p.name,
            city: p.city || "",
            score: p.score || 0,
            distance: (p.distance || 0) / 1852 // Convert m to nm
        })));

        // 3. Update Settlements
        const rawSettlements = this.props.settlements.get() || {};
        const settleList = rawSettlements.labels || [];
        console.log(`PhileasPage: Raw Settlements count: ${settleList.length}`);
        this.uiSettlements.set(settleList.slice(0, 50).map((s: any) => ({
            name: s.name,
            pop: s.pop,
            distance: 0 // populate if available
        })));
    }

    public onDestroy(): void {
        this.subscriptions.forEach(s => s.destroy && s.destroy());
        this.uiTelemetry.length; // Access property to silence unused warning if any
    }

    private setTab(tab: string) {
        this.activeTab.set(tab);
    }

    private renderPoiItem = (item: PoiItem): VNode => {
        return (
            <div class="list-row poi-row">
                <div class="col-name">{item.name}</div>
                <div class="col-city">{item.city}</div>
                <div class="col-dist">{item.distance.toFixed(1)}nm</div>
                <div class="col-score">{item.score.toFixed(1)}</div>
            </div>
        );
    }

    private renderSettlementItem = (item: SettlementItem): VNode => {
        return (
            <div class="list-row settlement-row">
                <div class="col-name">{item.name}</div>
                <div class="col-pop">{item.pop.toLocaleString()}</div>
            </div>
        );
    }

    public render(): TVNode<HTMLDivElement> {
        return (
            <div class="phileas-page">
                {/* Top Padding */}
                <div class="status-bar-spacer"></div>

                {/* Toolbar */}
                <div class="phileas-toolbar">
                    <div class="brand">Phileas <span class="version">v{VERSION}</span></div>
                    <TTButton key="Map" callback={() => this.setTab('map')} />
                    <TTButton key="Dashboard" callback={() => this.setTab('dashboard')} />
                    <TTButton key="POIs" callback={() => this.setTab('pois')} />
                    <TTButton key="Cities" callback={() => this.setTab('settlements')} />
                </div>

                {/* Content */}
                <div class="phileas-content">

                    {/* Map View */}
                    <div ref={this.mapContainerRef} class="view-container" style="display: block;">
                        <MapComponent
                            bus={this.props.bus}
                            telemetry={this.props.telemetry}
                            pois={this.props.pois}
                            settlements={this.props.settlements}
                            isVisible={this.isMapVisible}
                        />
                    </div>

                    {/* Dashboard */}
                    <div ref={this.dashboardContainerRef} class="view-container scrollable" style="display: none;">
                        <div class="info-card">
                            <h3>Aircraft Position</h3>
                            <div class="telemetry-grid">
                                <div><strong>Lat:</strong> {this.telemDisplay.lat}</div>
                                <div><strong>Lon:</strong> {this.telemDisplay.lon}</div>
                                <div><strong>Alt:</strong> {this.telemDisplay.alt} ft</div>
                                <div><strong>Hdg:</strong> {this.telemDisplay.hdg}°</div>
                            </div>
                        </div>
                        <div class="info-card">
                            <h3>System</h3>
                            <div>Built: {BUILD_TIMESTAMP.replace('T', ' ').substring(0, 16)}</div>
                        </div>
                    </div>

                    {/* POIs List */}
                    <div ref={this.poisContainerRef} class="view-container scrollable" style="display: none;">
                        <h2>Tracked POIs</h2>
                        <div class="list-header">
                            <div class="col-name">Name</div>
                            <div class="col-city">City</div>
                            <div class="col-dist">Dist</div>
                            <div class="col-score">Score</div>
                        </div>
                        <List data={this.uiPois} renderItem={this.renderPoiItem} class="efb-list" refreshOnUpdate={true} />
                    </div>

                    {/* Settlements List */}
                    <div ref={this.settlementsContainerRef} class="view-container scrollable" style="display: none;">
                        <h2>Settlements</h2>
                        <div class="list-header">
                            <div class="col-name">Name</div>
                            <div class="col-pop">Population</div>
                        </div>
                        <List data={this.uiSettlements} renderItem={this.renderSettlementItem} class="efb-list" refreshOnUpdate={true} />
                    </div>

                </div>
            </div>
        );
    }
}
