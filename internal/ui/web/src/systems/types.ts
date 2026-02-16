import type { Telemetry } from '../types/telemetry';
import type { POI } from '../hooks/usePOIs';
import type { LabelDTO } from '../types/mapLabels';
import type { MapFrame } from '../types/artisticMap';

export type MapMode = 'IDLE' | 'FLIGHT' | 'REPLAY' | 'TRANSITION';

export interface MapContext {
    telemetry: Telemetry | null;
    pois: POI[];
    zoom: number;
    center: [number, number];
    mode: MapMode;
    progress: number;
    validEvents: any[];
    poiDiscoveryTimes: Map<string, number>;
    totalTripTime: number;
    firstEventTime: number;
    narratorStatus: any;
    settlementCategories: string[];
    beaconMaxTargets: number;
    stabilizedReplay: boolean;
    mapWidth: number;
    mapHeight: number;
    styleLoaded: boolean;
    fontsLoaded: boolean;
    randomLocation?: [number, number];
}

export interface SystemState {
    frame: MapFrame;
    accumulatedSettlements: Map<string, LabelDTO>;
    accumulatedPois: Map<string, POI>;
}

export interface IMapSystem {
    update(dt: number, ctx: MapContext, state: SystemState): void;
    reset(): void;
}
