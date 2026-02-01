import { useEffect, useState } from 'react';
import type { POI } from '../hooks/usePOIs';

interface OverlayPOIPanelProps {
    poi: POI | null;
    title?: string;
    currentType?: string;
    playbackProgress: number; // 0-1
    isPlaying: boolean;
}

const getName = (poi: POI) => {
    if (poi.name_user) return poi.name_user;
    if (poi.name_en) return poi.name_en;
    return poi.name_local || 'Unknown';
};

export const OverlayPOIPanel = ({ poi, title, currentType, playbackProgress, isPlaying }: OverlayPOIPanelProps) => {
    const [thumbnailUrl, setThumbnailUrl] = useState<string | null>(null);
    const [visible, setVisible] = useState(false);

    useEffect(() => {
        if ((poi || title) && isPlaying) {
            setVisible(true);

            if (poi) {
                // Fetch thumbnail if not available
                if (poi.thumbnail_url) {
                    setThumbnailUrl(poi.thumbnail_url);
                } else {
                    // Fetch on-demand
                    fetch(`/api/pois/${poi.wikidata_id}/thumbnail`)
                        .then(res => res.json())
                        .then(data => {
                            if (data.url) setThumbnailUrl(data.url);
                        })
                        .catch(() => { });
                }
            } else {
                setThumbnailUrl(null);
            }
        } else {
            setVisible(false);
            setThumbnailUrl(null);
        }
    }, [poi, title, isPlaying]);

    if (!poi && !title) return null;

    let primaryName = title || "Narration";
    let category = "";

    if (poi) {
        primaryName = getName(poi);
        category = poi.specific_category || poi.category || '';
    } else if (currentType === 'debriefing') {
        category = "Flight Summary";
    } else if (currentType === 'essay') {
        category = "Regional Essay";
    }

    return (
        <div className={`overlay-poi-panel ${visible ? 'visible' : ''}`}>
            <div className="poi-name">{primaryName}</div>
            <div className="poi-category">{category}</div>

            {thumbnailUrl && (
                <img
                    src={thumbnailUrl}
                    alt={primaryName}
                    className="poi-thumbnail"
                />
            )}

            <div className="progress-container">
                <div
                    className="progress-bar"
                    style={{ width: `${playbackProgress * 100}%` }}
                />
            </div>
        </div>
    );
};
