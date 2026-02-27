import { useState, useEffect, useRef } from 'react';
import type { Telemetry } from '../types/telemetry';

export interface Settlement {
    id: string;
    name: string;
    lat: number;
    lon: number;
    pop: number;
    category: string;
}

export function useSettlements(telemetry: Telemetry | null | undefined) {
    const [settlements, setSettlements] = useState<Settlement[]>([]);
    const lastPosRef = useRef<{ lat: number; lon: number } | null>(null);

    useEffect(() => {
        const fetchSettlements = async () => {
            if (!telemetry || !telemetry.Valid) return;

            // Simple BBox around aircraft
            const bbox = [
                telemetry.Latitude - 0.5,
                telemetry.Longitude - 0.5,
                telemetry.Latitude + 0.5,
                telemetry.Longitude + 0.5
            ];

            try {
                const response = await fetch('/api/map/labels/sync?sid=web', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        bbox,
                        ac_lat: telemetry.Latitude,
                        ac_lon: telemetry.Longitude,
                        heading: telemetry.Heading,
                        zoom: 10
                    })
                });

                if (!response.ok) return;
                const data = await response.json();
                setSettlements(data.labels || []);
                lastPosRef.current = { lat: telemetry.Latitude, lon: telemetry.Longitude };
            } catch (error) {
                console.error("Failed to fetch settlements", error);
            }
        };

        // Initial fetch if we have telemetry
        if (telemetry?.Valid) {
            fetchSettlements();
        }

        // Poll every 5 seconds
        const interval = setInterval(fetchSettlements, 5000);
        return () => clearInterval(interval);
    }, [telemetry?.Latitude, telemetry?.Longitude, telemetry?.Valid]); // Re-run if position changes significantly or telemetry becomes valid

    return settlements;
}
