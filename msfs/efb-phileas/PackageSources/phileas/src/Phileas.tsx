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
import { Logger } from "./Utils/Logger";

import "./Phileas.scss";

declare const BASE_URL: string;
declare const VERSION: string;
declare const BUILD_TIMESTAMP: string;

class PhileasAppView extends AppView<RequiredProps<AppViewProps, "bus">> {
  protected defaultView = "MainPage";

  private telemetry = Subject.create<any>(null);
  private pois = Subject.create<any[]>([]);
  private settlements = Subject.create<any[]>([]);
  private updateTimer: any = null;

  protected registerViews(): void {
    this.appViewService.registerPage("MainPage", () => (
      <PhileasPage
        appViewService={this.appViewService}
        bus={this.props.bus}
        telemetry={this.telemetry}
        pois={this.pois}
        settlements={this.settlements}
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
      console.log("Phileas: Polling backend...");
      const telResponse = await fetch("http://127.0.0.1:1920/api/telemetry");
      if (telResponse.ok) {
        const telData = await telResponse.json();
        console.log("Phileas: Telemetry received", telData.Valid);
        if (telData.Valid) {
          const telemetry = telData;
          this.telemetry.set(telemetry);

          const poisResponse = await fetch("http://127.0.0.1:1920/api/pois/tracked");
          if (poisResponse.ok) {
            const poisData = await poisResponse.json();
            console.log(`Phileas: POIs received: ${poisData.length}`);
            this.pois.set(poisData);
          }

          const lat = telemetry.Latitude;
          const lon = telemetry.Longitude;
          const range = 0.5;
          const body = {
            BBox: [lat - range, lon - range, lat + range, lon + range],
            ACLat: lat,
            ACLon: lon,
            Zoom: 10
          };

          const setResponse = await fetch("http://127.0.0.1:1920/api/map/labels/sync", {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body)
          });
          if (setResponse.ok) {
            const setData = await setResponse.json();
            console.log(`Phileas: Settlements received: ${(setData.labels || []).length}`);
            this.settlements.set(setData);
          }
        }
      } else {
        console.warn(`Phileas: Telemetry fetch failed: ${telResponse.status}`);
      }
    } catch (err) {
      console.error("Phileas: Fetch error:", err);
    }
    this.updateTimer = setTimeout(() => this.loop(), 5000);
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
