import type { SyncRequest, SyncResponse, LabelDTO } from '../types/mapLabels';

export const labelService = {
    async fetchLabels(req: SyncRequest): Promise<LabelDTO[]> {
        const url = req.sid
            ? `/api/map/labels/sync?sid=${encodeURIComponent(req.sid)}`
            : '/api/map/labels/sync';
        const response = await fetch(url, {
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
