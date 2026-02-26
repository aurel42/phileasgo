import { useQuery } from '@tanstack/react-query';

export interface BackendStats {
    providers: Record<string, {
        api_success: number;
        api_errors: number;
        api_zero?: number;
        free_tier?: boolean;
        cache_hits?: number;
        cache_misses?: number;
        hit_rate?: number;
    }>;
    llm_fallback?: string[];
    tracking?: {
        active_pois: number;
    };
    diagnostics?: Array<{
        name: string;
        memory_mb: number;
        memory_max_mb: number;
        cpu_sec: number;
        cpu_max_sec: number;
    }>;
    go_mem?: {
        heap_alloc_mb: number;
        heap_inuse_mb: number;
        heap_idle_mb: number;
        heap_sys_mb: number;
        stack_inuse_mb: number;
        gc_sys_mb: number;
        other_sys_mb: number;
        mspan_inuse_mb: number;
        mcache_inuse_mb: number;
        total_sys_mb: number;
        num_goroutine: number;
        heap_objects: number;
        num_gc: number;
    };
}

export interface BackendVersion {
    version: string;
}

const fetchStats = async (): Promise<BackendStats> => {
    const res = await fetch('/api/stats');
    if (!res.ok) throw new Error('Failed to fetch stats');
    return res.json();
};

const fetchVersion = async (): Promise<BackendVersion> => {
    const res = await fetch('/api/version');
    if (!res.ok) throw new Error('Failed to fetch version');
    return res.json();
};

export const useBackendStats = () => {
    return useQuery({
        queryKey: ['backendStats'],
        queryFn: fetchStats,
        refetchInterval: 5000,
    });
};

export const useBackendVersion = () => {
    return useQuery({
        queryKey: ['backendVersion'],
        queryFn: fetchVersion,
        refetchInterval: 60000, // Version doesn't change often
        staleTime: Infinity,
    });
};
