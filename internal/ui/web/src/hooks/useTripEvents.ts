import { useQuery } from '@tanstack/react-query';

export interface TripEvent {
    timestamp: string;
    type: string;
    category?: string;
    title?: string;
    summary?: string;
    metadata?: Record<string, string>;
    lat: number;
    lon: number;
}

const fetchTripEvents = async (): Promise<TripEvent[]> => {
    const response = await fetch('/api/trip/events');
    if (!response.ok) {
        throw new Error('Failed to fetch trip events');
    }
    return response.json();
};

export const useTripEvents = () => {
    return useQuery({
        queryKey: ['tripEvents'],
        queryFn: fetchTripEvents,
        staleTime: 60_000, // Consider data fresh for 1 minute
        refetchInterval: false, // Don't auto-refetch
    });
};
