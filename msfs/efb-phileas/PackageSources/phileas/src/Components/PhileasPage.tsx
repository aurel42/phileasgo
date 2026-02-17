import { TTButton, GamepadUiView, RequiredProps, TVNode, UiViewProps, List } from "@efb/efb-api";
import { FSComponent, Subject, VNode, ArraySubject, EventBus } from "@microsoft/msfs-sdk";
import { MapComponent } from "./MapComponent";

import "./PhileasPage.scss";

declare const VERSION: string;
declare const BUILD_TIMESTAMP: string;

interface PhileasPageProps extends RequiredProps<UiViewProps, "appViewService"> {
    bus: EventBus;
    telemetry: Subject<any>;
    pois: Subject<any[]>;
    settlements: Subject<any>;
    apiVersion: Subject<string>;
    apiStats: Subject<any>;
    geography: Subject<any>;
}

interface PoiItem {
    name: string;
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

    // Throttled Subjects for UI Binding
    private readonly uiPois = ArraySubject.create<PoiItem>([]);
    private readonly uiSettlements = ArraySubject.create<SettlementItem>([]);

    private readonly isMapVisible = Subject.create<boolean>(true);

    // BY DESIGN: UI Component Update Frequencies - maintained for readability/performance balance
    private readonly DASHBOARD_INTERVAL = 2000; // Dashboard: 2s
    private readonly LIST_INTERVAL = 5000;      // POIs/Cities/Map Overlay: 5s

    private lastDashboardUpdate = 0;
    private lastListUpdate = 0;
    private subscriptions: any[] = [];

    // Refs for visibility control
    private readonly mapContainerRef = FSComponent.createRef<HTMLDivElement>();
    private readonly dashboardContainerRef = FSComponent.createRef<HTMLDivElement>();
    private readonly poisContainerRef = FSComponent.createRef<HTMLDivElement>();
    private readonly settlementsContainerRef = FSComponent.createRef<HTMLDivElement>();

    // Geographic Display
    private readonly geoDisplay = {
        main: Subject.create<string>("Locating..."),
        sub: Subject.create<string>("")
    };

    public onAfterRender(): void {
        // Tab switching logic
        this.activeTab.sub(tab => {
            const isMap = tab === 'map';
            if (this.isMapVisible.get() !== isMap) {
                this.isMapVisible.set(isMap);
            }

            if (this.mapContainerRef.instance) this.mapContainerRef.instance.style.display = tab === 'map' ? 'block' : 'none';
            if (this.dashboardContainerRef.instance) this.dashboardContainerRef.instance.style.display = tab === 'dashboard' ? 'block' : 'none';
            if (this.poisContainerRef.instance) this.poisContainerRef.instance.style.display = tab === 'pois' ? 'block' : 'none';
            if (this.settlementsContainerRef.instance) this.settlementsContainerRef.instance.style.display = tab === 'settlements' ? 'block' : 'none';

            this.updateUiData(true);
        });

        // Subscribe to raw data props
        this.subscriptions.push(this.props.telemetry.sub(() => this.updateUiData(false)));
        this.subscriptions.push(this.props.pois.sub(() => this.updateUiData(false)));
        this.subscriptions.push(this.props.settlements.sub(() => this.updateUiData(false)));
        this.subscriptions.push(this.props.geography.sub(() => this.updateUiData(false)));

        // Initial update
        this.updateUiData(true);
    }

    private updateUiData(force: boolean) {
        const now = Date.now();
        const shouldUpdateDashboard = force || (now - this.lastDashboardUpdate >= this.DASHBOARD_INTERVAL);
        const shouldUpdateLists = force || (now - this.lastListUpdate >= this.LIST_INTERVAL);

        const t = this.props.telemetry.get();

        // 1. Update Geography Display & Dashboard (2s)
        if (shouldUpdateDashboard) {
            this.lastDashboardUpdate = now;
            const geo = this.props.geography.get();
            if (geo) {
                if (geo.city) {
                    this.geoDisplay.main.set(geo.city === 'Unknown' ? "Far from civilization" : `near ${geo.city}`);
                    this.geoDisplay.sub.set(`${geo.city_region ? `${geo.city_region}, ` : ''}${geo.city_country}`);
                } else if (geo.country) {
                    this.geoDisplay.main.set(geo.country);
                    this.geoDisplay.sub.set(geo.region || "");
                }
            }
        }

        // 2. Update POIs & Cities (5s)
        if (shouldUpdateLists) {
            this.lastListUpdate = now;

            // Update POIs (Calculate distance on frontend)
            const rawPois = this.props.pois.get() || [];
            const sortedPois = [...rawPois].map((p: any) => {
                let dist = 0;
                if (t && p.lat !== undefined && p.lon !== undefined) {
                    dist = this.calculateDistance(t.Latitude, t.Longitude, p.lat, p.lon);
                }
                return {
                    name: p.name_user || p.name_en || p.name_local || p.wikidata_id || "Unknown POI",
                    score: p.score ?? p.Score ?? 0,
                    distance: dist
                };
            }).sort((a, b) => a.distance - b.distance).slice(0, 50);
            this.uiPois.set(sortedPois);

            // Update Settlements (Calculate distance & Sort)
            const rawSettlements = this.props.settlements.get() || {};
            const settleList = (rawSettlements as any).labels || [];
            const mappedSettlements = settleList.map((s: any) => {
                let dist = 0;
                if (t && s.lat !== undefined && s.lon !== undefined) {
                    dist = this.calculateDistance(t.Latitude, t.Longitude, s.lat, s.lon);
                }
                return {
                    name: s.name || s.city_name || "Unknown City",
                    pop: s.pop ?? s.population ?? 0,
                    distance: dist
                };
            }).sort((a: any, b: any) => a.distance - b.distance).slice(0, 50);
            this.uiSettlements.set(mappedSettlements);
        }
    }

    private calculateDistance(lat1: number, lon1: number, lat2: number, lon2: number): number {
        const p = 0.017453292519943295; // Math.PI / 180
        const c = Math.cos;
        const a = 0.5 - c((lat2 - lat1) * p) / 2 +
            c(lat1 * p) * c(lat2 * p) *
            (1 - c((lon2 - lon1) * p)) / 2;
        return 12742 * Math.asin(Math.sqrt(a)) / 1.852; // NM
    }

    public onDestroy(): void {
        this.subscriptions.forEach(s => s.destroy && s.destroy());
    }

    private setTab(tab: string) {
        this.activeTab.set(tab);
    }

    private renderPoiItem = (item: PoiItem): VNode => {
        return (
            <div class="list-row poi-row">
                <div class="col-name">{item.name}</div>
                <div class="col-dist">{item.distance.toFixed(1)}nm</div>
                <div class="col-score">{item.score.toFixed(1)}</div>
            </div>
        );
    }

    private renderSettlementItem = (item: SettlementItem): VNode => {
        return (
            <div class="list-row settlement-row">
                <div class="col-name">{item.name}</div>
                <div class="col-dist">{item.distance.toFixed(1)}nm</div>
                <div class="col-pop">{item.pop.toLocaleString()}</div>
            </div>
        );
    }

    private renderStats = (): VNode | null => {
        const stats = this.props.apiStats.get();
        if (!stats || !stats.providers) return null;

        return (
            <div class="info-card stats-card-grid">
                <h3>API Statistics</h3>
                <div class="stats-grid">
                    {Object.entries(stats.providers).map(([key, data]: [string, any]) => {
                        const hasActivity = (data.api_success || 0) + (data.api_errors || 0) > 0;
                        if (!hasActivity) return null;
                        return (
                            <div class="stat-entry">
                                <span class="stat-label">{key.toUpperCase()}</span>
                                <span class="stat-value">
                                    <span class="success">{data.api_success}</span> / <span class="error">{data.api_errors}</span>
                                </span>
                            </div>
                        );
                    })}
                </div>
            </div>
        );
    }

    private renderDiagnostics = (): VNode | null => {
        const stats = this.props.apiStats.get();
        if (!stats || !stats.diagnostics) return null;

        return (
            <div class="info-card system-card">
                <h3>System Diagnostics</h3>
                <table class="diagnostics-table">
                    <thead>
                        <tr>
                            <th>Process</th>
                            <th>Mem</th>
                            <th>CPU</th>
                        </tr>
                    </thead>
                    <tbody>
                        {stats.diagnostics.map((d: any) => (
                            <tr key={d.name}>
                                <td>{d.name}</td>
                                <td>{d.memory_mb}MB</td>
                                <td>{d.cpu_sec.toFixed(2)}</td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>
        );
    }

    public render(): TVNode<HTMLDivElement> {
        return (
            <div ref={this.gamepadUiViewRef} class="phileas-page">
                <div class="status-bar-spacer"></div>

                <div class="phileas-toolbar">
                    <div class="brand">Phileas <span class="version">{this.props.apiVersion}</span></div>
                    <TTButton key="Map" callback={() => this.setTab('map')} />
                    <TTButton key="Dashboard" callback={() => this.setTab('dashboard')} />
                    <TTButton key="POIs" callback={() => this.setTab('pois')} />
                    <TTButton key="Cities" callback={() => this.setTab('settlements')} />
                </div>

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
                    <div ref={this.dashboardContainerRef} class="view-container scrollable no-telemetry" style="display: none;">
                        <div class="info-card location-card">
                            <h3>Current Location</h3>
                            <div class="geo-main">{this.geoDisplay.main}</div>
                            <div class="geo-sub">{this.geoDisplay.sub}</div>
                        </div>

                        {this.renderStats()}
                        {this.renderDiagnostics()}

                        <div class="info-card built-card">
                            <h3>Build Information</h3>
                            <div>Version: {this.props.apiVersion}</div>
                            <div>Build: {BUILD_TIMESTAMP.replace('T', ' ').substring(0, 16)}</div>
                        </div>
                    </div>

                    {/* POIs List */}
                    <div ref={this.poisContainerRef} class="view-container scrollable" style="display: none;">
                        <h2>Tracked POIs</h2>
                        <div class="list-header po-header">
                            <div class="col-name">Name</div>
                            <div class="col-dist">Dist</div>
                            <div class="col-score">Score</div>
                        </div>
                        <List data={this.uiPois} renderItem={this.renderPoiItem} class="efb-list" refreshOnUpdate={true} />
                    </div>

                    {/* Settlements List */}
                    <div ref={this.settlementsContainerRef} class="view-container scrollable" style="display: none;">
                        <h2>Settlements</h2>
                        <div class="list-header settle-header">
                            <div class="col-name">Name</div>
                            <div class="col-dist">Dist</div>
                            <div class="col-pop">Pop</div>
                        </div>
                        <List data={this.uiSettlements} renderItem={this.renderSettlementItem} class="efb-list" refreshOnUpdate={true} />
                    </div>

                </div>
            </div>
        );
    }
}
