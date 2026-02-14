export interface Telemetry {
    Latitude: number;
    Longitude: number;
    AltitudeMSL: number;
    AltitudeAGL: number;
    Heading: number;
    GroundSpeed: number;
    VerticalSpeed: number;
    IsOnGround: boolean;
    FlightStage?: string;
    APStatus?: string;
    ValleyAltitude?: number; // In Meters
    SimState: 'active' | 'inactive' | 'disconnected';
    Valid: boolean;
    Provider?: string;
}
