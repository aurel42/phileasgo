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
    // Visibility
    AmbientInCloud?: number;
    AmbientVisibility?: number;
    SimState: 'active' | 'inactive' | 'disconnected';
}
