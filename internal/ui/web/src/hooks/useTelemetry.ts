import { useQuery } from '@tanstack/react-query';
import type { Telemetry } from '../types/telemetry';

const fetchTelemetry = async (): Promise<Telemetry> => {
    const response = await fetch('/api/telemetry');
    if (!response.ok) {
        throw new Error('Network response was not ok');
    }
    return response.json();
};

export const useTelemetry = () => {
    return useQuery({
        queryKey: ['telemetry'],
        queryFn: fetchTelemetry,
        refetchInterval: 500, // Poll every 500ms
        retry: false,
    });
};
