import { useState, useEffect, useRef } from 'react';
import type { Telemetry } from '../types/telemetry';

export interface Geography {
    city: string;
    region?: string;
    country: string;
    city_region?: string;
    city_country?: string;
    country_code?: string;
    city_country_code?: string;
}

export const useGeography = (telemetry?: Telemetry) => {
    const [location, setLocation] = useState<Geography | null>(null);
    const telemetryRef = useRef(telemetry);

    useEffect(() => {
        telemetryRef.current = telemetry;
    }, [telemetry]);

    useEffect(() => {
        const fetchLocation = () => {
            const t = telemetryRef.current;
            // Only fetch if telemetry exists AND is marked as Valid
            // This prevents fetching "International Waters" for (0,0) when sim is undefined/loading
            if (!t || !t.Valid) {
                setLocation(null);
                return;
            }

            fetch(`/api/geography?lat=${t.Latitude}&lon=${t.Longitude}`)
                .then(r => r.json())
                .then(data => setLocation(data))
                .catch(() => { });
        };

        fetchLocation();
        const interval = setInterval(fetchLocation, 10000);

        return () => clearInterval(interval);
    }, []); // Empty dependency array means this effect runs once on mount + creates the interval

    return { location };
};
