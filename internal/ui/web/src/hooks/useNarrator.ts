import { useQuery } from '@tanstack/react-query';
import type { NarratorStatusResponse } from '../types/narrator';

const fetchNarratorStatus = async (): Promise<NarratorStatusResponse> => {
    const response = await fetch('/api/narrator/status');
    if (!response.ok) {
        throw new Error('Failed to fetch narrator status');
    }
    return response.json();
};

export const useNarrator = () => {
    const query = useQuery({
        queryKey: ['narratorStatus'],
        queryFn: fetchNarratorStatus,
        refetchInterval: 1000,
        retry: false,
    });

    return {
        status: query.data,
        isLoading: query.isLoading,
        isError: query.isError,
    };
};
