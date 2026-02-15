import type { POI } from '../hooks/usePOIs';
import type { Telemetry } from '../types/telemetry';
import type { LabelCandidate } from '../metrics/PlacementEngine';
import type { Feature } from 'geojson';
import type { AircraftType } from '../components/AircraftIcon';

export interface ArtisticMapProps {
    className?: string;
    center: [number, number];
    zoom: number;
    telemetry: Telemetry | null;
    pois: POI[];
    settlementTier: number;
    settlementCategories: string[];
    paperOpacityFog: number;
    paperOpacityClear: number;
    parchmentSaturation: number;
    selectedPOI?: POI | null;
    isAutoOpened?: boolean;
    onPOISelect: (poi: POI) => void;
    onMapClick: () => void;
    beaconMaxTargets?: number;
    showDebugBoxes?: boolean;
    // Aircraft Configuration
    aircraftIcon?: AircraftType;
    aircraftSize?: number;
    aircraftColorMain?: string;
    aircraftColorAccent?: string;
    // Map Injection for testing
    mapFactory?: (options: any) => any;
}

// Single Atomic Frame state for strict synchronization
export interface MapFrame {
    labels: LabelCandidate[];
    maskPath: string;
    center: [number, number];
    zoom: number;
    offset: [number, number];
    heading: number;
    bearingLine: Feature<any> | null;
    aircraftX: number;
    aircraftY: number;
    agl: number;
}
