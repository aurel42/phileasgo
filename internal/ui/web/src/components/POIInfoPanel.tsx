
import { useEffect, useState } from 'react';
import type { POI } from '../hooks/usePOIs';
import { useQueryClient } from '@tanstack/react-query';
import type { AudioStatus } from '../types/audio';
import { useNarrator } from '../hooks/useNarrator';
import { getPOIDisplayName } from '../utils/poiUtils';

interface POIInfoPanelProps {
    poi: POI | null;
    pois: POI[];  // Fresh POI list from polling
    aircraftHeading: number;
    currentTitle?: string;
    currentType?: string;
    onClose: () => void;
}

const getColor = (score: number) => {
    const clamped = Math.max(1, Math.min(50, score));
    const ratio = (clamped - 1) / 49;
    const hue = 60 - (ratio * 60);
    return `hsl(${hue}, 100%, 50%)`;
};

const getName = getPOIDisplayName;

const getLocalNameIfDifferent = (poi: POI, primaryName: string) => {
    if (poi.name_local && poi.name_local !== primaryName) {
        return poi.name_local;
    }
    return null;
};

const formatTimeAgo = (dateStr: string) => {
    const date = new Date(dateStr);
    const now = new Date();
    const seconds = Math.floor((now.getTime() - date.getTime()) / 1000);

    if (seconds < 60) return 'Just now';
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours}h ago`;
    const days = Math.floor(hours / 24);
    return `${days}d ago`;
};

export const POIInfoPanel = ({ poi, pois, currentTitle, currentType, onClose }: POIInfoPanelProps) => {
    const [thumbnailUrl, setThumbnailUrl] = useState<string | null>(null);
    const [strategy, setStrategy] = useState<'min_skew' | 'uniform' | 'max_skew'>('min_skew');
    const queryClient = useQueryClient();
    const { status: narratorStatus } = useNarrator();

    // Get fresh POI data from the polled pois array
    const freshPoi = pois.find(p => p.wikidata_id === poi?.wikidata_id);
    const thumbnailFromData = freshPoi?.thumbnail_url || poi?.thumbnail_url;

    useEffect(() => {
        if (!poi) {
            setThumbnailUrl(null);
            return;
        }

        // Sync narration strategy if available in fresh data
        if (freshPoi?.narration_strategy) {
            setStrategy(freshPoi.narration_strategy as 'min_skew' | 'uniform' | 'max_skew');
        }

        // 1. Use thumbnail from fresh data if available
        if (thumbnailFromData) {
            setThumbnailUrl(thumbnailFromData);
            return;
        }

        // 2. Fallback: Fetch thumbnail on-demand from API
        const fetchThumbnail = async () => {
            try {
                const res = await fetch(`/api/pois/${poi.wikidata_id}/thumbnail`);
                if (res.ok) {
                    const data = await res.json();
                    if (data.url) {
                        setThumbnailUrl(data.url);
                    }
                }
            } catch (e) {
                console.error("Failed to fetch thumbnail", e);
            }
        };

        fetchThumbnail();
    }, [poi, thumbnailFromData, freshPoi?.narration_strategy]);

    // If no POI, we are in a generic nomination mode (Debrief, Essay, Screenshot)
    if (!poi) {
        let displayTitle = currentTitle;
        let displayCategory = 'Regional Essay';
        let defaultText = 'Enjoy this regional essay about your current surroundings.';

        if (currentType === 'debriefing') {
            displayTitle = displayTitle || 'Flight Debrief';
            displayCategory = 'Flight Summary';
            defaultText = 'Your flight has concluded. Listen to the automated pilot debrief.';
        } else if (currentType === 'screenshot') {
            displayTitle = displayTitle || 'Photograph Analysis';
            displayCategory = 'Visual Reconnaissance';
            defaultText = 'Analyzing the captured imagery with local geographical context.';
        } else {
            displayTitle = displayTitle || 'Regional Essay';
        }

        const screenshotUrl = (currentType === 'screenshot' && narratorStatus?.current_image_path)
            ? `/api/images/serve?path=${encodeURIComponent(narratorStatus.current_image_path)}`
            : null;

        return (
            <div className="poi-info-panel generic-narration" style={{ display: 'flex', gap: '12px' }}>
                <div style={{ flex: screenshotUrl ? '1' : '1 1 auto', minWidth: 0 }}>
                    <div className="panel-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                        <div className="role-label">{displayCategory}</div>
                        <button className="close-btn role-btn" onClick={onClose} style={{ background: 'transparent', border: 'none', color: '#666', fontSize: '20px', cursor: 'pointer' }}>×</button>
                    </div>
                    <div className="poi-details">
                        <h1 className="role-title" style={{ margin: '8px 0' }}>{displayTitle}</h1>
                        <p className="role-text-lg" style={{ opacity: 0.8 }}>
                            {defaultText}
                        </p>
                    </div>
                </div>
                {screenshotUrl && (
                    <div style={{ flex: '0 0 60%', minWidth: 0 }}>
                        <img
                            src={screenshotUrl}
                            alt="Screenshot Analysis"
                            className="poi-thumbnail"
                            style={{
                                width: '100%',
                                height: 'auto',
                                boxShadow: '0 4px 15px rgba(0,0,0,0.5)',
                                border: '1px solid rgba(255,255,255,0.1)'
                            }}
                        />
                    </div>
                )}
            </div>
        );
    }

    if (!poi) return null;

    const primaryName = getName(poi);
    const localName = getLocalNameIfDifferent(poi, primaryName);

    // Check for duplication with PlaybackControls
    const isNarratingThis = narratorStatus?.current_poi?.wikidata_id === poi.wikidata_id &&
        (narratorStatus?.playback_status === 'playing' || narratorStatus?.playback_status === 'paused' || narratorStatus?.playback_status === 'preparing');

    const handlePlay = async () => {
        // Optimistic update
        queryClient.setQueryData(['audioStatus'], (old: AudioStatus | undefined) => {
            if (!old) return old;
            return {
                ...old,
                is_playing: true,
                title: 'Loading: ' + primaryName,
                // zero out progress
                position: 0,
                duration: 0
            };
        });

        try {
            await fetch('/api/narrator/play', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    poi_id: poi.wikidata_id,
                    strategy: strategy
                })
            });
            // Force immediate refetch to get real state if it happened fast
            queryClient.invalidateQueries({ queryKey: ['audioStatus'] });
        } catch (e) {
            console.error("Failed to trigger play", e);
            // Revert on error? Handled by next poll usually.
            queryClient.invalidateQueries({ queryKey: ['audioStatus'] });
        }
    };

    return (
        <div className="hud-container poi-info-panel" style={{ position: 'relative', flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0 }}>
            {/* Close button (absolute top-right) */}
            <button
                onClick={onClose}
                style={{
                    position: 'absolute',
                    top: '8px',
                    right: '8px',
                    background: 'transparent',
                    border: 'none',
                    color: '#666',
                    fontSize: '20px',
                    cursor: 'pointer',
                    padding: '0 4px',
                    lineHeight: 1,
                    zIndex: 10,
                }}
            >
                &times;
            </button>

            {/* Main layout: Text on left, Thumbnail on right */}
            <div style={{ display: 'flex', gap: '12px', flex: 1, minHeight: 0 }}>
                {/* Left column: Text content (40%) */}
                <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>

                    {/* Title (Hidden if currently narrating this POI to avoid duplication with PlaybackControls) */}
                    {!isNarratingThis && (
                        <div className="role-title" style={{ marginBottom: '8px', lineHeight: '1.2' }}>
                            {primaryName}
                        </div>
                    )}

                    {/* Meta / Controls */}
                    <div style={{ marginBottom: '4px', display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
                        <div style={{ display: 'flex', alignItems: 'center' }}>
                            <button
                                onClick={(e) => {
                                    console.log("Play button clicked for", poi.wikidata_id);
                                    e.stopPropagation();
                                    handlePlay();
                                }}
                                title="Play Narration"
                                style={{
                                    background: 'transparent',
                                    border: '1px solid var(--accent)',
                                    borderRadius: '50%',
                                    width: '24px',
                                    height: '24px',
                                    display: 'flex',
                                    alignItems: 'center',
                                    justifyContent: 'center',
                                    color: 'var(--accent)',
                                    cursor: 'pointer',
                                    flexShrink: 0,
                                    zIndex: 20, /* Ensure it's clickable */
                                    position: 'relative' /* Context for z-index */
                                }}
                            >
                                ▶
                            </button>
                            <div className="length-selector">
                                <button
                                    className={`length-btn role-btn ${strategy === 'min_skew' ? 'active' : ''}`}
                                    onClick={(e) => { e.stopPropagation(); setStrategy('min_skew'); }}
                                    title="Short Narration"
                                >SHORT</button>
                                <button
                                    className={`length-btn role-btn ${strategy === 'max_skew' ? 'active' : ''}`}
                                    onClick={(e) => { e.stopPropagation(); setStrategy('max_skew'); }}
                                    title="Long Narration"
                                >LONG</button>
                            </div>
                        </div>
                    </div>
                    {localName && (
                        <div className="role-text-sm" style={{ fontStyle: 'italic', marginBottom: '4px' }}>
                            ({localName})
                        </div>
                    )}
                    <div className="role-label" style={{ marginBottom: '8px' }}>
                        {poi.category}
                        {poi.specific_category && poi.specific_category !== poi.category && (
                            <span className="role-text-sm" style={{ opacity: 0.7 }}> ({poi.specific_category})</span>
                        )}
                    </div>

                    {/* Info */}
                    <div className="status-pill role-btn" style={{ display: 'inline-flex', alignItems: 'center', marginBottom: '8px', flexShrink: 0 }}>
                        <div className="status-dot connected" style={{ backgroundColor: getColor(poi.score) }}></div>
                        <span style={{ color: 'var(--text-color)' }}>Score: <span className="role-num-sm">{poi.score?.toFixed(1)}</span></span>
                    </div>

                    {poi.last_played && poi.last_played !== "0001-01-01T00:00:00Z" && (
                        <div className="role-text-sm" style={{ opacity: 0.7, marginBottom: '8px' }}>
                            Last Played: {formatTimeAgo(poi.last_played)}
                        </div>
                    )}

                    {poi.wp_url && (
                        <div style={{ marginBottom: '8px' }}>
                            <a href={poi.wp_url} target="_blank" rel="noopener noreferrer" style={{ color: 'var(--accent)', textDecoration: 'none' }}>
                                Wikipedia Article &rarr;
                            </a>
                        </div>
                    )}

                    {/* Score Details */}
                    {poi.score_details && (
                        <div style={{ marginTop: '4px', flex: '1 1 auto', display: 'flex', flexDirection: 'column', minHeight: 0 }}>
                            <div className="role-header" style={{ marginBottom: '4px', flexShrink: 0 }}>Score Breakdown</div>
                            <div className="role-label" style={{
                                opacity: 0.8,
                                whiteSpace: 'pre-wrap',
                                overflowY: 'auto',
                                background: 'rgba(0,0,0,0.2)',
                                padding: '6px',
                                borderRadius: '4px',
                                border: '1px solid rgba(255,255,255,0.05)',
                                flex: 1
                            }}>
                                {poi.score_details}
                            </div>
                        </div>
                    )}
                </div>

                {/* Right column: Thumbnail (60% basis) */}
                {thumbnailUrl && (
                    <div style={{ flex: '0 0 60%' }}>
                        <img
                            src={thumbnailUrl}
                            alt={primaryName}
                            className="poi-thumbnail"
                            style={{
                                width: '100%',
                                height: 'auto',
                            }}
                        />
                    </div>
                )}
            </div>
        </div >
    );
};
