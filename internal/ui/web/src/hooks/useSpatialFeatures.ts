import { useQuery } from '@tanstack/react-query';

export interface SpatialFeature {
    qid: string;
    name: string;
    category: string;
}

/**
 * Fetches the currently active spatial features from the backend.
 * Coverage reflects the aircraft's current position against marine and regional datasets.
 */
export const useSpatialFeatures = () => {
    return useQuery<SpatialFeature[]>({
        queryKey: ['spatialFeatures'],
        queryFn: async () => {
            const response = await fetch('/api/features');
            if (!response.ok) {
                throw new Error('Network response was not ok');
            }
            return response.json();
        },
        refetchInterval: 5000,
        placeholderData: [],
    });
};
