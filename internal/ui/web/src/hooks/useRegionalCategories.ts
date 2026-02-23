import { useQuery } from '@tanstack/react-query';
import type { RegionalCategory } from '../types/regionalCategory';

/**
 * Fetches the currently active dynamic regional categories from the backend.
 * Polls the backend to determine if the location-aware context job has
 * discovered new features in the current area.
 */
export const useRegionalCategories = () => {
    return useQuery<RegionalCategory[]>({
        queryKey: ['regionalCategories'],
        queryFn: async () => {
            const response = await fetch('/api/regional');
            if (!response.ok) {
                throw new Error('Network response was not ok');
            }
            return response.json();
        },
        refetchInterval: 5000,
        placeholderData: [], // Prevent layout flashes by defaulting to empty array
    });
};
