export interface LabelDTO {
    id: string;
    lat: number;
    lon: number;
    name: string;
    pop: number;
    category: string;
}

export interface SyncRequest {
    bbox: number[]; // [minLat, minLon, maxLat, maxLon]
    ac_lat: number;
    ac_lon: number;
    heading: number;
    zoom: number;
}

export interface SyncResponse {
    labels: LabelDTO[];
}
