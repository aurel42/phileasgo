import { TTButton, GamepadUiView, RequiredProps, TVNode, UiViewProps, List, Switch, Slider, Incremental } from "@efb/efb-api";
import { FSComponent, Subject, VNode, ArraySubject, EventBus } from "@microsoft/msfs-sdk";
import { MapComponent } from "./MapComponent";

import "./PhileasPage.scss";

function isPoiOnCooldown(poi: any): boolean {
    return !!poi.is_on_cooldown;
}

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
    narratorStatus: Subject<any>;
    aircraftConfig: Subject<any>;
}

interface PoiItem {
    name: string;
    score: number;
    distance: number;
    cooldown: boolean;
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

    // Internal state for Settings Tab
    private readonly settingPaused = Subject.create<boolean>(false);
    private readonly settingFreq = Subject.create<number>(3);
    private readonly settingLength = Subject.create<number>(3);
    private readonly settingFilterMode = Subject.create<string>("fixed");
    private readonly settingMinScore = Subject.create<number>(0.5);
    private readonly settingTargetCount = Subject.create<number>(20);
    private settingsSyncing = false;

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
    private readonly settingsContainerRef = FSComponent.createRef<HTMLDivElement>();

    // Dashboard card refs + cached mutable elements
    private readonly statsCardRef = FSComponent.createRef<HTMLDivElement>();
    private readonly diagnosticsCardRef = FSComponent.createRef<HTMLDivElement>();
    private statsCells = new Map<string, { success: HTMLSpanElement; errors: HTMLSpanElement }>();
    private diagCells = new Map<string, { mem: HTMLTableCellElement; cpu: HTMLTableCellElement }>();

    // Overlay: refs from render(), cached text elements populated in onAfterRender()
    private readonly overlayPlayingRef = FSComponent.createRef<HTMLDivElement>();
    private readonly overlayPreparingRef = FSComponent.createRef<HTMLDivElement>();
    private readonly overlayLocationRef = FSComponent.createRef<HTMLDivElement>();
    private readonly overlayStatusRef = FSComponent.createRef<HTMLDivElement>();

    // Cached text nodes inside overlay lines — updated via textContent, never innerHTML
    private overlayPlayingText: Text | null = null;
    private overlayPreparingText: Text | null = null;
    private overlayGeoLoc: HTMLSpanElement | null = null;
    private overlayStatusDot: HTMLDivElement | null = null;
    private overlayStatusLabel: HTMLSpanElement | null = null;
    private overlayPoiStatus: HTMLSpanElement | null = null;
    private overlayPoiActive: HTMLSpanElement | null = null;
    private overlayPoiCooldown: HTMLSpanElement | null = null;
    private overlayPoiTracked: HTMLSpanElement | null = null;

    private readonly frqPips: HTMLDivElement[] = [];
    private readonly lenPips: HTMLDivElement[] = [];

    public onAfterRender(node: VNode): void {
        super.onAfterRender(node);

        // Tab switching logic
        this.subscriptions.push(this.activeTab.sub(tab => {
            const isMap = tab === 'map';
            if (this.isMapVisible.get() !== isMap) {
                this.isMapVisible.set(isMap);
            }

            if (this.mapContainerRef.instance) this.mapContainerRef.instance.style.display = tab === 'map' ? 'block' : 'none';
            if (this.dashboardContainerRef.instance) this.dashboardContainerRef.instance.style.display = tab === 'dashboard' ? 'block' : 'none';
            if (this.poisContainerRef.instance) this.poisContainerRef.instance.style.display = tab === 'pois' ? 'block' : 'none';
            if (this.settlementsContainerRef.instance) this.settlementsContainerRef.instance.style.display = tab === 'settlements' ? 'block' : 'none';
            if (this.settingsContainerRef.instance) this.settingsContainerRef.instance.style.display = tab === 'settings' ? 'block' : 'none';

            if (tab === 'dashboard') this.updateDashboardCards();
            this.updateUiData(true);
        }));

        // Subscribe to raw data props
        this.subscriptions.push(this.props.telemetry.sub(() => this.updateUiData(false)));
        this.subscriptions.push(this.props.pois.sub(() => this.updateUiData(false)));
        this.subscriptions.push(this.props.settlements.sub(() => this.updateUiData(false)));
        this.subscriptions.push(this.props.geography.sub(() => this.updateUiData(false)));
        this.subscriptions.push(this.props.apiStats.sub(() => {
            if (this.activeTab.get() === 'dashboard') this.updateDashboardCards();
        }));

        // Settings synchronization
        this.subscriptions.push(this.props.aircraftConfig.sub(cfg => {
            if (!cfg) return;
            this.settingsSyncing = true;
            this.settingFreq.set(cfg.narration_frequency ?? 3);
            this.settingLength.set(cfg.text_length ?? 3);
            this.settingFilterMode.set(cfg.filter_mode || 'fixed');
            this.settingMinScore.set(cfg.min_poi_score ?? 0.5);
            this.settingTargetCount.set(cfg.target_poi_count ?? 20);
            this.settingsSyncing = false;
        }));
        this.subscriptions.push(this.props.narratorStatus.sub(status => {
            if (!status) return;
            this.settingsSyncing = true;
            this.settingPaused.set(status.is_user_paused ?? false);
            this.settingsSyncing = false;
        }));

        this.subscriptions.push(this.settingPaused.sub(val => { if (!this.settingsSyncing) this.updateBackendConfig('paused', val); }));
        this.subscriptions.push(this.settingFreq.sub(val => { if (!this.settingsSyncing) this.updateBackendConfig('narration_frequency', val); }));
        this.subscriptions.push(this.settingLength.sub(val => { if (!this.settingsSyncing) this.updateBackendConfig('text_length', val); }));
        this.subscriptions.push(this.settingFilterMode.sub(val => { if (!this.settingsSyncing) this.updateBackendConfig('filter_mode', val); }));
        this.subscriptions.push(this.settingMinScore.sub(val => { if (!this.settingsSyncing) this.updateBackendConfig('min_poi_score', val); }));
        this.subscriptions.push(this.settingTargetCount.sub(val => { if (!this.settingsSyncing) this.updateBackendConfig('target_poi_count', val); }));

        // Build overlay DOM structure once — subsequent updates only touch textContent
        this.buildOverlayDom();

        // Initial update
        this.updateUiData(true);
    }

    private updateUiData(force: boolean) {
        const now = Date.now();
        const shouldUpdateDashboard = force || (now - this.lastDashboardUpdate >= this.DASHBOARD_INTERVAL);
        const shouldUpdateLists = force || (now - this.lastListUpdate >= this.LIST_INTERVAL);

        const t = this.props.telemetry.get();

        // 1. Update Dashboard & Overlay (2s)
        if (shouldUpdateDashboard) {
            this.lastDashboardUpdate = now;
        }

        // Update Overlay if on Map tab (same 2s cadence)
        if (shouldUpdateDashboard && this.activeTab.get() === 'map') {
            this.updateStatusOverlay();
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
                    distance: dist,
                    cooldown: isPoiOnCooldown(p),
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

    public destroy(): void {
        this.subscriptions.forEach(s => s.destroy && s.destroy());
        super.destroy();
    }

    private setTab(tab: string) {
        this.activeTab.set(tab);
        if (tab === 'settings') {
            // Force immediate config refresh when opening settings
            fetch("http://127.0.0.1:1920/api/config")
                .then(r => r.json())
                .then(data => this.props.aircraftConfig.set(data))
                .catch(() => { });
        }
    }

    private async updateBackendConfig(key: string, value: any) {
        if (key === 'paused') {
            fetch("http://127.0.0.1:1920/api/audio/control", {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ action: value ? 'pause' : 'resume' })
            }).catch(e => console.error("Failed to update pause state", e));
            return;
        }

        fetch("http://127.0.0.1:1920/api/config", {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ [key]: value })
        }).catch(e => console.error(`Failed to update config ${key}`, e));
    }

    private renderPoiItem = (item: PoiItem): VNode => {
        return (
            <div class="list-row poi-row">
                <div class={`col-name ${item.cooldown ? 'poi-cooldown' : 'poi-active'}`}>{item.name}</div>
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

    private updateDashboardCards(): void {
        const stats = this.props.apiStats.get();
        if (this.activeTab.get() === 'dashboard') {
            this.updateStatsCard(stats);
            this.updateDiagnosticsCard(stats);
        }
    }

    /** Build overlay DOM once — cache element refs for fast textContent updates. */
    private buildOverlayDom(): void {
        // Playing line: <span class="label">Playing:</span> <text>
        const playingEl = this.overlayPlayingRef.instance;
        if (playingEl) {
            const label = document.createElement('span');
            label.className = 'label';
            label.textContent = 'Playing:';
            this.overlayPlayingText = document.createTextNode(' ');
            playingEl.appendChild(label);
            playingEl.appendChild(this.overlayPlayingText);
        }

        // Preparing line: <span class="label">Preparing:</span> <text>
        const preparingEl = this.overlayPreparingRef.instance;
        if (preparingEl) {
            const label = document.createElement('span');
            label.className = 'label';
            label.textContent = 'Preparing:';
            this.overlayPreparingText = document.createTextNode(' ');
            preparingEl.appendChild(label);
            preparingEl.appendChild(this.overlayPreparingText);
        }

        // Location line: near **City**, Admin1, Country [in Accent]
        const locationEl = this.overlayLocationRef.instance;
        if (locationEl) {
            this.overlayGeoLoc = document.createElement('span');
            this.overlayGeoLoc.className = 'geo-loc';
            locationEl.appendChild(this.overlayGeoLoc);
        }

        // Status pill line
        const statusEl = this.overlayStatusRef.instance;
        if (statusEl) {
            const pill = document.createElement('div');
            pill.className = 'status-pill';
            this.overlayStatusDot = document.createElement('div');
            this.overlayStatusDot.className = 'status-dot';
            this.overlayStatusLabel = document.createElement('span');
            this.overlayStatusLabel.className = 'status-label';
            pill.appendChild(this.overlayStatusDot);
            pill.appendChild(this.overlayStatusLabel);
            statusEl.appendChild(pill);

            // POI Status — build colored spans once, update textContent later
            const poiStatus = document.createElement('div');
            poiStatus.className = 'poi-status';
            this.overlayPoiStatus = document.createElement('span');

            this.overlayPoiActive = document.createElement('span');
            this.overlayPoiActive.className = 'poi-count-active';
            this.overlayPoiCooldown = document.createElement('span');
            this.overlayPoiCooldown.className = 'poi-count-cooldown';
            this.overlayPoiTracked = document.createElement('span');
            this.overlayPoiTracked.className = 'poi-count-tracked';

            this.overlayPoiStatus.appendChild(document.createTextNode('POI '));
            this.overlayPoiStatus.appendChild(this.overlayPoiActive);
            this.overlayPoiStatus.appendChild(document.createTextNode(' | '));
            this.overlayPoiStatus.appendChild(this.overlayPoiCooldown);
            this.overlayPoiStatus.appendChild(document.createTextNode(' | POI(tracked) '));
            this.overlayPoiStatus.appendChild(this.overlayPoiTracked);

            poiStatus.appendChild(this.overlayPoiStatus);
            statusEl.appendChild(poiStatus);

            // Settings Pips (FRQ)
            const frq = document.createElement('div');
            frq.className = 'settings-group';
            const frqLabel = document.createElement('span');
            frqLabel.className = 'label';
            frqLabel.textContent = 'FRQ ';
            const frqPipsCont = document.createElement('div');
            frqPipsCont.className = 'pip-container';
            for (let i = 0; i < 4; i++) {
                const pip = document.createElement('div');
                pip.className = 'pip';
                frqPipsCont.appendChild(pip);
                this.frqPips.push(pip);
            }
            frq.append(frqLabel, frqPipsCont);
            statusEl.appendChild(frq);

            // Settings Pips (LEN)
            const len = document.createElement('div');
            len.className = 'settings-group';
            const lenLabel = document.createElement('span');
            lenLabel.className = 'label';
            lenLabel.textContent = 'LEN ';
            const lenPipsCont = document.createElement('div');
            lenPipsCont.className = 'pip-container';
            for (let i = 0; i < 5; i++) {
                const pip = document.createElement('div');
                pip.className = 'pip';
                lenPipsCont.appendChild(pip);
                this.lenPips.push(pip);
            }
            len.append(lenLabel, lenPipsCont);
            statusEl.appendChild(len);
        }
    }

    /** Update overlay text content — no innerHTML, no DOM tree reconstruction. */
    private updateStatusOverlay(): void {
        const narrator = this.props.narratorStatus.get();
        const playing = narrator?.current_poi;
        const preparing = narrator?.preparing_poi;

        // Playing
        const playingEl = this.overlayPlayingRef.instance;
        if (playingEl && this.overlayPlayingText) {
            if (playing) {
                this.overlayPlayingText.textContent =
                    ' ' + (playing.name_user || playing.name_en || playing.name_local || playing.wikidata_id);
                playingEl.style.display = 'block';
            } else {
                playingEl.style.display = 'none';
            }
        }

        // Preparing
        const preparingEl = this.overlayPreparingRef.instance;
        if (preparingEl && this.overlayPreparingText) {
            if (preparing) {
                this.overlayPreparingText.textContent =
                    ' ' + (preparing.name_user || preparing.name_en || preparing.name_local || preparing.wikidata_id);
                preparingEl.style.display = 'block';
            } else {
                preparingEl.style.display = 'none';
            }
        }

        // Location (Single Line) — DOM APIs only, no innerHTML
        if (this.overlayGeoLoc) {
            const geo = this.props.geography.get();
            if (geo) {
                const el = this.overlayGeoLoc;
                el.textContent = '';
                if (geo.city) {
                    if (geo.city === 'Unknown') {
                        el.appendChild(document.createTextNode('Far from civilization'));
                    } else {
                        el.appendChild(document.createTextNode('near '));
                        const strong = document.createElement('strong');
                        strong.textContent = geo.city;
                        el.appendChild(strong);
                    }

                    // Cross-border or region/country
                    const suffix: string[] = [];
                    if (geo.city_country_code && geo.country_code && geo.city_country_code !== geo.country_code) {
                        if (geo.city_region) suffix.push(geo.city_region);
                        suffix.push(geo.city_country);
                        suffix.push(`[in ${geo.country}]`);
                    } else {
                        if (geo.region) suffix.push(geo.region);
                        if (geo.country) suffix.push(geo.country);
                    }
                    if (suffix.length) {
                        el.appendChild(document.createTextNode(', ' + suffix.join(', ')));
                    }
                } else if (geo.country) {
                    el.textContent = geo.region ? `${geo.region}, ${geo.country}` : geo.country;
                }
            }
        }

        // Status pills & Dots
        const stats = this.props.apiStats.get();
        if (this.overlayStatusDot && this.overlayStatusLabel) {
            const tel = this.props.telemetry.get();
            const simStateStr = tel?.SimState || 'disconnected';

            let statusClass = 'disconnected';
            if (simStateStr === 'active') {
                statusClass = 'sim-running';
            } else if (simStateStr === 'inactive') {
                statusClass = 'connected';
            }

            this.overlayStatusDot.className = `status-dot ${statusClass}`;
            this.overlayStatusLabel.textContent = `SIM ${simStateStr.toUpperCase()}`;
        }

        // POI Counts — update cached spans, no DOM rebuild
        if (this.overlayPoiActive && this.overlayPoiCooldown && this.overlayPoiTracked) {
            const rawPois = this.props.pois.get() || [];
            const active = rawPois.filter(p => !isPoiOnCooldown(p)).length;
            const cooldown = rawPois.filter(p => isPoiOnCooldown(p)).length;
            const tracked = stats?.tracking?.active_pois || 0;

            this.overlayPoiActive.textContent = String(active);
            this.overlayPoiCooldown.textContent = String(cooldown);
            this.overlayPoiTracked.textContent = String(tracked);
        }

        // Settings Pips
        const frq = narrator?.narration_frequency ?? 0;
        const len = narrator?.text_length ?? 0;

        this.frqPips.forEach((p, i) => {
            p.className = `pip ${frq > i ? 'active' : ''} ${frq > i && i >= 2 ? 'high' : ''}`;
        });
        this.lenPips.forEach((p, i) => {
            p.className = `pip ${len > i ? 'active' : ''} ${len > i && i >= 4 ? 'high' : ''}`;
        });
    }

    private updateStatsCard(stats: any): void {
        const el = this.statsCardRef.instance;
        if (!el) return;

        if (!stats?.providers) { el.style.display = 'none'; return; }

        const active = Object.entries(stats.providers)
            .filter(([, d]: [string, any]) => (d.api_success || 0) + (d.api_errors || 0) > 0);
        if (active.length === 0) { el.style.display = 'none'; return; }

        // Check if provider set changed (rebuild needed)
        const keys = active.map(([k]) => k).sort().join(',');
        const cachedKeys = [...this.statsCells.keys()].sort().join(',');

        if (keys !== cachedKeys) {
            el.innerHTML = '';
            this.statsCells.clear();
            const h3 = document.createElement('h3'); h3.textContent = 'API Statistics';
            const grid = document.createElement('div'); grid.className = 'stats-grid';
            for (const [key, data] of active as [string, any][]) {
                const entry = document.createElement('div'); entry.className = 'stat-entry';
                const label = document.createElement('span'); label.className = 'stat-label';
                label.textContent = key.toUpperCase();
                const value = document.createElement('span'); value.className = 'stat-value';
                const suc = document.createElement('span'); suc.className = 'success';
                const err = document.createElement('span'); err.className = 'error';
                suc.textContent = String(data.api_success ?? 0);
                err.textContent = String(data.api_errors ?? 0);
                value.append(suc, ' / ', err);
                entry.append(label, value);
                grid.appendChild(entry);
                this.statsCells.set(key, { success: suc, errors: err });
            }
            el.append(h3, grid);
        } else {
            for (const [key, data] of active as [string, any][]) {
                const cached = this.statsCells.get(key);
                if (cached) {
                    cached.success.textContent = String(data.api_success ?? 0);
                    cached.errors.textContent = String(data.api_errors ?? 0);
                }
            }
        }
        el.style.display = '';
    }

    private updateDiagnosticsCard(stats: any): void {
        const el = this.diagnosticsCardRef.instance;
        if (!el) return;

        if (!stats?.diagnostics?.length) { el.style.display = 'none'; return; }

        const names = stats.diagnostics.map((d: any) => d.name).sort().join(',');
        const cachedNames = [...this.diagCells.keys()].sort().join(',');

        if (names !== cachedNames) {
            el.innerHTML = '';
            this.diagCells.clear();
            const h3 = document.createElement('h3'); h3.textContent = 'System Diagnostics';
            const table = document.createElement('table'); table.className = 'diagnostics-table';
            const thead = document.createElement('thead');
            thead.innerHTML = '<tr><th>Process</th><th>Mem</th><th>CPU</th></tr>';
            const tbody = document.createElement('tbody');
            for (const d of stats.diagnostics) {
                const tr = document.createElement('tr');
                const tdName = document.createElement('td'); tdName.textContent = d.name;
                const tdMem = document.createElement('td'); tdMem.textContent = `${d.memory_mb}MB`;
                const tdCpu = document.createElement('td'); tdCpu.textContent = d.cpu_sec.toFixed(2);
                tr.append(tdName, tdMem, tdCpu);
                tbody.appendChild(tr);
                this.diagCells.set(d.name, { mem: tdMem, cpu: tdCpu });
            }
            table.append(thead, tbody);
            el.append(h3, table);
        } else {
            for (const d of stats.diagnostics) {
                const cached = this.diagCells.get(d.name);
                if (cached) {
                    cached.mem.textContent = `${d.memory_mb}MB`;
                    cached.cpu.textContent = d.cpu_sec.toFixed(2);
                }
            }
        }
        el.style.display = '';
    }

    private renderSettingsView(): VNode {
        const freqLabels = ['Rarely', 'Normal', 'Active', 'Hyperactive'];
        const lenLabels = ['Short', 'Brief', 'Normal', 'Detailed', 'Long'];

        return (
            <div class="settings-view">
                <h2>Narration Settings</h2>

                <div class="settings-row">
                    <div class="settings-label">Pause Narration</div>
                    <div class="settings-control">
                        <Switch checked={this.settingPaused} />
                    </div>
                </div>

                <div class="settings-row">
                    <div class="settings-label">Frequency</div>
                    <div class="settings-control">
                        <Incremental
                            value={this.settingFreq}
                            min={1} max={4} step={1}
                            formatter={(v) => freqLabels[v - 1] || String(v)}
                            useTextbox={false}
                        />
                    </div>
                </div>

                <div class="settings-row">
                    <div class="settings-label">Script Length</div>
                    <div class="settings-control">
                        <Incremental
                            value={this.settingLength}
                            min={1} max={5} step={1}
                            formatter={(v) => lenLabels[v - 1] || String(v)}
                            useTextbox={false}
                        />
                    </div>
                </div>

                <h2 style="margin-top: 20px;">POI Selection</h2>

                <div class="settings-row">
                    <div class="settings-label">Selection Mode</div>
                    <div class="settings-control mode-toggle">
                        <TTButton
                            key={this.settingFilterMode.map(m => m === 'fixed' ? 'Fixed Score' : 'Adaptive Count')}
                            callback={() => this.settingFilterMode.set(this.settingFilterMode.get() === 'fixed' ? 'adaptive' : 'fixed')}
                        />
                    </div>
                </div>

                <div class="settings-row" style={this.settingFilterMode.map(m => m === 'fixed' ? '' : 'display:none')}>
                    <div class="settings-label">Min Score</div>
                    <div class="settings-control">
                        <div class="slider-value-box">{this.settingMinScore.map(s => s.toFixed(1))}</div>
                        <Slider value={this.settingMinScore} min={-10} max={10} step={0.5} />
                    </div>
                </div>
                <div class="settings-row" style={this.settingFilterMode.map(m => m === 'adaptive' ? '' : 'display:none')}>
                    <div class="settings-label">Target Count</div>
                    <div class="settings-control">
                        <div class="slider-value-box">{this.settingTargetCount}</div>
                        <Slider value={this.settingTargetCount} min={5} max={50} step={1} />
                    </div>
                </div>
            </div>
        );
    }

    public render(): TVNode<HTMLDivElement> {
        return (
            <div ref={this.gamepadUiViewRef} class="phileas-page">
                <div class="status-bar-spacer"></div>

                <div class="phileas-toolbar">
                    <div class="brand">Phileas&nbsp;<span class="version">{this.props.apiVersion}</span></div>
                    <TTButton key="Map" callback={() => this.setTab('map')} selected={this.activeTab.map(t => t === 'map')} />
                    <TTButton key="POIs" callback={() => this.setTab('pois')} selected={this.activeTab.map(t => t === 'pois')} />
                    <TTButton key="Cities" callback={() => this.setTab('settlements')} selected={this.activeTab.map(t => t === 'settlements')} />
                    <TTButton key="Settings" callback={() => this.setTab('settings')} selected={this.activeTab.map(t => t === 'settings')} />
                    <TTButton key="System" callback={() => this.setTab('dashboard')} selected={this.activeTab.map(t => t === 'dashboard')} />
                </div>

                <div class="phileas-content">
                    {/* Map View */}
                    <div ref={this.mapContainerRef} class="view-container" style="display: block; padding: 0;">
                        <MapComponent
                            bus={this.props.bus}
                            telemetry={this.props.telemetry}
                            pois={this.props.pois}
                            settlements={this.props.settlements}
                            isVisible={this.isMapVisible}
                            narratorStatus={this.props.narratorStatus}
                            aircraftConfig={this.props.aircraftConfig}
                        />

                        {/* Status Overlay */}
                        <div class="phileas-overlay">
                            <div ref={this.overlayPlayingRef} class="status-line playing-line" style="display: none;" />
                            <div ref={this.overlayPreparingRef} class="status-line preparing-line" style="display: none;" />
                            <div ref={this.overlayLocationRef} class="status-line location-line" />
                            <div ref={this.overlayStatusRef} class="status-line status-pill-row" />
                        </div>
                    </div>

                    {/* System (formerly Dashboard) */}
                    <div ref={this.dashboardContainerRef} class="view-container scrollable no-telemetry" style="display: none;">
                        <div ref={this.statsCardRef} class="info-card stats-card-grid" style="display: none;" />
                        <div ref={this.diagnosticsCardRef} class="info-card system-card" style="display: none;" />

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

                    {/* Settings View */}
                    <div ref={this.settingsContainerRef} class="view-container scrollable no-telemetry" style="display: none;">
                        {this.renderSettingsView()}
                    </div>

                </div>
            </div >
        );
    }
}
