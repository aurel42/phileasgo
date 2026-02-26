import {
  App,
  AppBootMode,
  AppInstallProps,
  AppSuspendMode,
  AppView,
  AppViewProps,
  Efb,
  RequiredProps,
  TVNode,
} from "@efb/efb-api";
import { FSComponent, VNode, Subject } from "@microsoft/msfs-sdk";
import { PhileasPage } from "./Components/PhileasPage";

import "./Phileas.scss";

declare const BASE_URL: string;
declare const VERSION: string;
declare const BUILD_TIMESTAMP: string;

class PhileasAppView extends AppView<RequiredProps<AppViewProps, "bus">> {
  protected defaultView = "MainPage";

  // Equality functions prevent downstream subscriber cascades when poll data is unchanged
  private telemetry = Subject.create<any>(null, (a: any, b: any) =>
    a === b || (a && b && a.Latitude === b.Latitude && a.Longitude === b.Longitude
      && a.Heading === b.Heading && a.Altitude === b.Altitude && a.AltitudeAGL === b.AltitudeAGL
      && a.GroundSpeed === b.GroundSpeed && a.OnGround === b.OnGround));
  private pois = Subject.create<any[]>([], (a: any[], b: any[]) =>
    a === b || (a.length === b.length && a.every((p, i) =>
      p.wikidata_id === b[i].wikidata_id && p.score === b[i].score
      && p.is_on_cooldown === b[i].is_on_cooldown && p.beacon_color === b[i].beacon_color)));
  private settlements = Subject.create<any[]>([]);
  private apiVersion = Subject.create<string>("v0.0.0");
  private apiStats = Subject.create<any>(null);
  private geography = Subject.create<any>(null, (a: any, b: any) =>
    a === b || (a && b && a.city === b.city && a.country === b.country
      && a.region === b.region && a.country_code === b.country_code
      && a.city_country_code === b.city_country_code));
  private narratorStatus = Subject.create<any>(null, (a: any, b: any) =>
    a === b || (a && b && a.current_poi?.wikidata_id === b.current_poi?.wikidata_id
      && a.preparing_poi?.wikidata_id === b.preparing_poi?.wikidata_id
      && a.narration_frequency === b.narration_frequency && a.text_length === b.text_length));

  private aircraftConfig = Subject.create<any>(null);
  private regionalCategories = Subject.create<any[]>([], (a: any[], b: any[]) =>
    a === b || (a.length === b.length && a.every((cat, i) => cat.qid === b[i].qid)));
  private updateTimer: any = null;
  private abortController: AbortController | null = null;
  private lastConfigFetch = 0;
  private readonly CONFIG_INTERVAL = 30000;
  private lastPoiFetch = 0;
  private readonly POI_INTERVAL = 5000;

  protected registerViews(): void {
    this.appViewService.registerPage("MainPage", () => (
      <PhileasPage
        appViewService={this.appViewService}
        bus={this.props.bus}
        telemetry={this.telemetry}
        pois={this.pois}
        settlements={this.settlements}
        apiVersion={this.apiVersion}
        apiStats={this.apiStats}
        geography={this.geography}
        narratorStatus={this.narratorStatus}
        aircraftConfig={this.aircraftConfig}
        regionalCategories={this.regionalCategories}
      />
    ));
  }

  public onOpen(): void {
    this.appViewService.open("MainPage");
    this.startLoop();
  }

  public onClose(): void {
    this.stopLoop();
  }

  public onResume(): void {
    this.startLoop();
  }

  public onPause(): void {
    this.stopLoop();
  }

  private startLoop(): void {
    if (this.updateTimer) return;
    this.abortController = new AbortController();
    this.loop();
  }

  private stopLoop(): void {
    if (this.abortController) {
      this.abortController.abort();
      this.abortController = null;
    }
    if (this.updateTimer) {
      clearTimeout(this.updateTimer);
      this.updateTimer = null;
    }
  }

  private async loop(): Promise<void> {
    const signal = this.abortController?.signal;
    try {
      const telResponse = await fetch("http://127.0.0.1:1920/api/telemetry", { signal });
      if (signal?.aborted) return;
      if (telResponse.ok) {
        const telData = await telResponse.json();
        if (telData.Valid) {
          const telemetry = telData;
          this.telemetry.set(telemetry);

          // Parallel fetches for efficiency
          const now = Date.now();
          const fetchConfig = now - this.lastConfigFetch >= this.CONFIG_INTERVAL;
          const fetchPois = now - this.lastPoiFetch >= this.POI_INTERVAL;

          const promises: Promise<Response>[] = [
            fetch("http://127.0.0.1:1920/api/map/labels/sync?sid=efb", {
              signal,
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                BBox: [telemetry.Latitude - 0.5, telemetry.Longitude - 0.5, telemetry.Latitude + 0.5, telemetry.Longitude + 0.5],
                ACLat: telemetry.Latitude,
                ACLon: telemetry.Longitude,
                Zoom: 10
              })
            }),
            fetch("http://127.0.0.1:1920/api/stats", { signal }),
            fetch("http://127.0.0.1:1920/api/version", { signal }),
            fetch(`http://127.0.0.1:1920/api/geography?lat=${telemetry.Latitude}&lon=${telemetry.Longitude}`, { signal }),
            fetch("http://127.0.0.1:1920/api/narrator/status", { signal }),
          ];
          if (fetchPois) {
            promises.push(fetch("http://127.0.0.1:1920/api/pois/tracked", { signal }));
            promises.push(fetch("http://127.0.0.1:1920/api/regional", { signal }));
          }
          if (fetchConfig) {
            promises.push(fetch("http://127.0.0.1:1920/api/config", { signal }));
          }

          const results = await Promise.all(promises);
          if (signal?.aborted) return;
          const [setRes, statsRes, verRes, geoRes, narRes] = results;

          if (setRes.ok) this.settlements.set(await setRes.json());
          if (statsRes.ok) this.apiStats.set(await statsRes.json());
          if (verRes.ok) {
            const v = await verRes.json();
            this.apiVersion.set(v.version);
          }
          if (geoRes.ok) this.geography.set(await geoRes.json());
          if (narRes.ok) this.narratorStatus.set(await narRes.json());
          let nextIdx = 5;
          if (fetchPois) {
            const poisRes = results[nextIdx++];
            if (poisRes.ok) this.pois.set(await poisRes.json());
            const regRes = results[nextIdx++];
            if (regRes.ok) this.regionalCategories.set(await regRes.json());
            this.lastPoiFetch = now;
          }
          if (fetchConfig) {
            const cfgRes = results[nextIdx++];
            if (cfgRes.ok) this.aircraftConfig.set(await cfgRes.json());
            this.lastConfigFetch = now;
          }
        }
      }
    } catch (err) {
      if (signal?.aborted) return;
      console.error("Phileas: Loop error");
    }
    // BY DESIGN: Main data loop frequency (1s) - maintained for responsive telemetry/data tracking
    this.updateTimer = setTimeout(() => this.loop(), 1000);
  }

  public render(): VNode {
    return <div class="phileas-app">{super.render()}</div>;
  }
}

class PhileasApp extends App {
  public get internalName(): string {
    return "phileas";
  }

  public get name(): string {
    return "Phileas";
  }

  public get icon(): string {
    return `${BASE_URL}/assets/app-icon.svg`;
  }

  public BootMode = AppBootMode.HOT;
  public SuspendMode = AppSuspendMode.SLEEP;

  public async install(_props: AppInstallProps): Promise<void> {
    Efb.loadCss(`${BASE_URL}/phileas.css`);
    return Promise.resolve();
  }

  public render(): TVNode<PhileasAppView> {
    return <PhileasAppView bus={this.bus} />;
  }
}

Efb.use(PhileasApp);
