import type { SyncRequest, SyncResponse, CheckShadowRequest, CheckShadowResponse, LabelDTO } from '../types/mapLabels';

export const labelService = {
    async fetchLabels(req: SyncRequest): Promise<LabelDTO[]> {
        const response = await fetch('/api/map/labels/sync', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(req)
        });

        if (!response.ok) {
            throw new Error(`Failed to fetch labels: ${response.statusText}`);
        }

        const data: SyncResponse = await response.json();
        return data.labels || [];
    }
};
