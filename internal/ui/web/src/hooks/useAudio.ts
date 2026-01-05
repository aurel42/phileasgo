import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import type { AudioStatus } from '../types/audio';

const fetchAudioStatus = async (): Promise<AudioStatus> => {
    const response = await fetch('/api/audio/status');
    if (!response.ok) {
        throw new Error('Failed to fetch audio status');
    }
    return response.json();
};

export const useAudio = () => {
    const queryClient = useQueryClient();

    const statusQuery = useQuery({
        queryKey: ['audioStatus'],
        queryFn: fetchAudioStatus,
        refetchInterval: 1000, // Poll every second
        retry: false,
    });

    const controlMutation = useMutation({
        mutationFn: async (action: string) => {
            const response = await fetch('/api/audio/control', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ action }),
            });
            if (!response.ok) {
                throw new Error('Failed to control audio');
            }
            return response.json();
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['audioStatus'] });
        },
    });

    const volumeMutation = useMutation({
        mutationFn: async (volume: number) => {
            const response = await fetch('/api/audio/volume', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ volume }),
            });
            if (!response.ok) {
                throw new Error('Failed to set volume');
            }
            return response.json();
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['audioStatus'] });
        },
    });

    return {
        status: statusQuery.data,
        isLoading: statusQuery.isLoading,
        isError: statusQuery.isError,
        control: controlMutation.mutate,
        setVolume: volumeMutation.mutate,
    };
};
