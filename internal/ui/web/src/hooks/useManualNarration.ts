import { useState, useCallback, useRef } from 'react';

export function useManualNarration() {
    const [statusMessage, setStatusMessage] = useState<string | null>(null);
    const feedbackTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

    const showFeedback = useCallback((msg: string) => {
        if (feedbackTimer.current) {
            clearTimeout(feedbackTimer.current);
        }
        setStatusMessage(msg);
        feedbackTimer.current = setTimeout(() => {
            setStatusMessage(null);
            feedbackTimer.current = null;
        }, 3000); // Clear after 3s
    }, []);

    const playPOI = useCallback(async (id: string, name: string) => {
        try {
            const res = await fetch('/api/narrator/play', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ poi_id: id })
            });
            if (res.ok) {
                showFeedback(`Narrating POI: ${name}`);
            }
        } catch (e) {
            console.error("Failed to trigger POI narration", e);
        }
    }, [showFeedback]);

    const playCity = useCallback(async (name: string) => {
        try {
            const res = await fetch('/api/narrator/play-city', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name })
            });
            if (res.ok) {
                showFeedback(`Searching for city: ${name}`);
            }
        } catch (e) {
            console.error("Failed to trigger city narration", e);
        }
    }, [showFeedback]);

    const playFeature = useCallback(async (qid: string, name: string) => {
        try {
            const res = await fetch('/api/narrator/play-feature', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ qid })
            });
            if (res.ok) {
                showFeedback(`Narrating feature: ${name}`);
            }
        } catch (e) {
            console.error("Failed to trigger feature narration", e);
        }
    }, [showFeedback]);

    return {
        playPOI,
        playCity,
        playFeature,
        statusMessage
    };
}
