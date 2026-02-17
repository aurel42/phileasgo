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

  private telemetry = Subject.create<any>(null);
  private pois = Subject.create<any[]>([]);
  private settlements = Subject.create<any[]>([]);
  private apiVersion = Subject.create<string>("v0.0.0");
  private apiStats = Subject.create<any>(null);
  private geography = Subject.create<any>(null);

  private updateTimer: any = null;

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
    this.loop();
  }

  private stopLoop(): void {
    if (this.updateTimer) {
      clearTimeout(this.updateTimer);
      this.updateTimer = null;
    }
  }

  private async loop(): Promise<void> {
    try {
      const telResponse = await fetch("http://127.0.0.1:1920/api/telemetry");
      if (telResponse.ok) {
        const telData = await telResponse.json();
        if (telData.Valid) {
          const telemetry = telData;
          this.telemetry.set(telemetry);

          // Parallel fetches for efficiency
          const [poisRes, setRes, statsRes, verRes, geoRes] = await Promise.all([
            fetch("http://127.0.0.1:1920/api/pois/tracked"),
            fetch("http://127.0.0.1:1920/api/map/labels/sync", {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                BBox: [telemetry.Latitude - 0.5, telemetry.Longitude - 0.5, telemetry.Latitude + 0.5, telemetry.Longitude + 0.5],
                ACLat: telemetry.Latitude,
                ACLon: telemetry.Longitude,
                Zoom: 10
              })
            }),
            fetch("http://127.0.0.1:1920/api/stats"),
            fetch("http://127.0.0.1:1920/api/version"),
            fetch(`http://127.0.0.1:1920/api/geography?lat=${telemetry.Latitude}&lon=${telemetry.Longitude}`)
          ]);

          if (poisRes.ok) this.pois.set(await poisRes.json());
          if (setRes.ok) this.settlements.set(await setRes.json());
          if (statsRes.ok) this.apiStats.set(await statsRes.json());
          if (verRes.ok) {
            const v = await verRes.json();
            this.apiVersion.set(v.version);
          }
          if (geoRes.ok) this.geography.set(await geoRes.json());
        }
      }
    } catch (err) {
      // Minimal error logging to prevent spam
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
